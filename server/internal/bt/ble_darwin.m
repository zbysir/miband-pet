// BLE 连接（CoreBluetooth）：手环在「连接新手机」的二维码界面时只能通过 BLE 连上。
// 帧格式和经典蓝牙一样（SPP v2），走 fe95 服务：往 0x5f 写，从 0x5e 收通知。
//
// CoreBluetooth 的回调都在主队列上。这里的函数都只是把操作 dispatch_async 到主队列然后立刻返回，
// 结果通过 goBle* 回调通知 Go，绝不在主线程上等结果（等就会和回调互相卡死）。

#import <Foundation/Foundation.h>
#import <CoreBluetooth/CoreBluetooth.h>
#include "ble_darwin.h"
#include "_cgo_export.h"

static CBUUID *uuidService(void) { return [CBUUID UUIDWithString:@"FE95"]; }
static CBUUID *uuidRX(void) { return [CBUUID UUIDWithString:@"0000005E-0000-1000-8000-00805F9B34FB"]; }
static CBUUID *uuidTX(void) { return [CBUUID UUIDWithString:@"0000005F-0000-1000-8000-00805F9B34FB"]; }

@interface MBBle : NSObject <CBCentralManagerDelegate, CBPeripheralDelegate>
@property(strong) CBCentralManager *manager;
@property(strong) NSData *macReversed;   // MiBeacon 服务数据里的 MAC 是倒序的
@property(strong) NSString *nameSuffix;  // 广播名字结尾是 MAC 后 4 位，如 "1D3F"
@property(assign) BOOL wantScan;
@property(assign) uintptr_t handle;
@property(strong) NSMutableDictionary<NSUUID *, CBPeripheral *> *seen;
@property(strong) CBPeripheral *peripheral;
@property(strong) CBCharacteristic *tx;
@property(strong) NSMutableArray<NSData *> *pending; // 等发送的分片（写缓冲满了就排队）
@end

@implementation MBBle

- (instancetype)init {
    self = [super init];
    self.seen = [NSMutableDictionary new];
    self.pending = [NSMutableArray new];
    self.manager = [[CBCentralManager alloc] initWithDelegate:self queue:nil];
    return self;
}

- (void)centralManagerDidUpdateState:(CBCentralManager *)central {
    if (central.state == CBManagerStatePoweredOn && self.wantScan) {
        [self startScan];
    }
}

- (void)startScan {
    if (self.manager.state != CBManagerStatePoweredOn) {
        return; // 等 centralManagerDidUpdateState
    }
    // 二维码界面的广播不一定带服务 UUID 列表，所以不按服务过滤，自己在回调里认
    [self.manager scanForPeripheralsWithServices:nil options:@{CBCentralManagerScanOptionAllowDuplicatesKey: @YES}];
}

- (void)centralManager:(CBCentralManager *)central didDiscoverPeripheral:(CBPeripheral *)p
     advertisementData:(NSDictionary<NSString *, id> *)ad RSSI:(NSNumber *)rssi {
    NSData *sd = ad[CBAdvertisementDataServiceDataKey][uuidService()];
    NSString *name = ad[CBAdvertisementDataLocalNameKey] ?: p.name;
    BOOL match = (sd != nil && self.macReversed != nil && [sd rangeOfData:self.macReversed options:0 range:NSMakeRange(0, sd.length)].location != NSNotFound)
                 || (name != nil && self.nameSuffix != nil && [name hasSuffix:self.nameSuffix] && [name containsString:@"Band"]);
    if (!match) {
        return;
    }
    self.seen[p.identifier] = p;
    goBleFound(self.handle, (char *)p.identifier.UUIDString.UTF8String, rssi.intValue);
}

- (void)centralManager:(CBCentralManager *)central didConnectPeripheral:(CBPeripheral *)p {
    p.delegate = self;
    [p discoverServices:@[uuidService()]];
}

- (void)centralManager:(CBCentralManager *)central didFailToConnectPeripheral:(CBPeripheral *)p error:(NSError *)error {
    goBleOpened(self.handle, -5, 0);
}

- (void)centralManager:(CBCentralManager *)central didDisconnectPeripheral:(CBPeripheral *)p error:(NSError *)error {
    if (p == self.peripheral) {
        self.peripheral = nil;
        self.tx = nil;
        [self.pending removeAllObjects];
        goBleClosed(self.handle);
    }
}

- (void)peripheral:(CBPeripheral *)p didDiscoverServices:(NSError *)error {
    for (CBService *s in p.services) {
        if ([s.UUID isEqual:uuidService()]) {
            [p discoverCharacteristics:@[uuidRX(), uuidTX()] forService:s];
            return;
        }
    }
    goBleOpened(self.handle, -6, 0); // 没有 fe95 服务
}

- (void)peripheral:(CBPeripheral *)p didDiscoverCharacteristicsForService:(CBService *)s error:(NSError *)error {
    CBCharacteristic *rx = nil;
    for (CBCharacteristic *c in s.characteristics) {
        if ([c.UUID isEqual:uuidRX()]) rx = c;
        if ([c.UUID isEqual:uuidTX()]) self.tx = c;
    }
    if (rx == nil || self.tx == nil) {
        goBleOpened(self.handle, -6, 0);
        return;
    }
    [p setNotifyValue:YES forCharacteristic:rx];
}

- (void)peripheral:(CBPeripheral *)p didUpdateNotificationStateForCharacteristic:(CBCharacteristic *)c error:(NSError *)error {
    if (error != nil) {
        goBleOpened(self.handle, -7, 0);
        return;
    }
    goBleOpened(self.handle, 0, (int)[p maximumWriteValueLengthForType:CBCharacteristicWriteWithoutResponse]);
}

- (void)peripheral:(CBPeripheral *)p didUpdateValueForCharacteristic:(CBCharacteristic *)c error:(NSError *)error {
    NSData *v = c.value;
    if (v.length > 0) {
        goBleData(self.handle, (void *)v.bytes, (int)v.length);
    }
}

- (void)pump {
    while (self.pending.count > 0 && self.peripheral != nil && self.tx != nil && self.peripheral.canSendWriteWithoutResponse) {
        NSData *chunk = self.pending.firstObject;
        [self.pending removeObjectAtIndex:0];
        [self.peripheral writeValue:chunk forCharacteristic:self.tx type:CBCharacteristicWriteWithoutResponse];
    }
}

- (void)peripheralIsReadyToSendWriteWithoutResponse:(CBPeripheral *)p {
    [self pump];
}

@end

static MBBle *gBle;

static void ensureBle(void) {
    if (gBle == nil) {
        gBle = [MBBle new];
    }
}

void ble_scan_start(const char *macReversed, int macLen, const char *nameSuffix, uintptr_t handle) {
    NSData *mac = [NSData dataWithBytes:macReversed length:macLen];
    NSString *suffix = [NSString stringWithUTF8String:nameSuffix];
    dispatch_async(dispatch_get_main_queue(), ^{
        ensureBle();
        gBle.macReversed = mac;
        gBle.nameSuffix = suffix;
        gBle.handle = handle;
        gBle.wantScan = YES;
        [gBle startScan];
    });
}

void ble_scan_stop(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gBle != nil) {
            gBle.wantScan = NO;
            [gBle.manager stopScan];
        }
    });
}

void ble_connect(const char *identifier, uintptr_t handle) {
    NSUUID *uuid = [[NSUUID alloc] initWithUUIDString:[NSString stringWithUTF8String:identifier]];
    dispatch_async(dispatch_get_main_queue(), ^{
        ensureBle();
        gBle.handle = handle;
        CBPeripheral *p = gBle.seen[uuid];
        if (p == nil) {
            p = [gBle.manager retrievePeripheralsWithIdentifiers:@[uuid]].firstObject;
        }
        if (p == nil) {
            goBleOpened(handle, -8, 0);
            return;
        }
        gBle.peripheral = p;
        [gBle.manager connectPeripheral:p options:nil];
    });
}

void ble_write(const void *data, int length, int chunk) {
    NSData *all = [NSData dataWithBytes:data length:length];
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gBle == nil) return;
        int n = chunk > 0 ? chunk : 20;
        for (int off = 0; off < length; off += n) {
            [gBle.pending addObject:[all subdataWithRange:NSMakeRange(off, MIN(n, length - off))]];
        }
        [gBle pump];
    });
}

void ble_disconnect(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gBle != nil && gBle.peripheral != nil) {
            [gBle.pending removeAllObjects];
            [gBle.manager cancelPeripheralConnection:gBle.peripheral];
        }
    });
}
