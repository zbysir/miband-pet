// macOS 经典蓝牙 RFCOMM 的最小封装：打开 SPP 通道、写字节、收字节回调给 Go。

#import <Foundation/Foundation.h>
#import <IOBluetooth/IOBluetooth.h>
#import <CoreBluetooth/CoreBluetooth.h>
#include "rfcomm_darwin.h"
#include "_cgo_export.h"

// IOBluetooth 的回调投递在主线程的 run loop 上，所以所有调用都派发到主线程执行，
// 主线程由 bt_run_main_loop 常驻 run loop（Go 侧在 main 里调 bt.RunMain）。
static void runOnBT(void (^block)(void)) {
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
}

void bt_run_main_loop(void) {
    CFRunLoopRun();
}

@interface MBConn : NSObject <IOBluetoothRFCOMMChannelDelegate>
@property(strong) IOBluetoothDevice *device;
@property(strong) IOBluetoothRFCOMMChannel *channel;
@property(assign) uintptr_t handle;
@property(assign) BOOL closed; // bt_close 之后晚到的回调一律不管（Go 那边的 handle 已经释放）
@end

// 打开连接分三步，全靠回调串起来，主线程上不等任何东西：
// 1. 打开底层连接（openConnection:withPageTimeout:，手环不在旁边时要好几秒才超时）
// 2. 找 SPP 服务记录（有缓存直接用，没有就异步做 SDP 查询）
// 3. 异步打开 RFCOMM 通道
// 任何一步失败或者成功打开，都通过 goRfcommOpened 告诉 Go。
@implementation MBConn
- (void)finish:(int)status mtu:(int)mtu {
    if (!self.closed) {
        goRfcommOpened(self.handle, status, mtu);
    }
}

- (void)start {
    if (self.device.isConnected) {
        [self findService];
        return;
    }
    // 页超时单位 0.625ms，0x2000 ≈ 5 秒
    IOReturn r = [self.device openConnection:self withPageTimeout:0x2000 authenticationRequired:NO];
    if (r != kIOReturnSuccess) {
        [self finish:r mtu:0];
    }
}

- (void)connectionComplete:(IOBluetoothDevice *)device status:(IOReturn)status {
    if (self.closed) {
        return;
    }
    if (status != kIOReturnSuccess) {
        [self finish:status mtu:0];
        return;
    }
    [self findService];
}

- (void)findService {
    IOBluetoothSDPServiceRecord *rec = [self.device getServiceRecordForUUID:[IOBluetoothSDPUUID uuid16:kBluetoothSDPUUID16ServiceClassSerialPort]];
    if (rec != nil) {
        [self openChannel:rec];
        return;
    }
    IOReturn r = [self.device performSDPQuery:self];
    if (r != kIOReturnSuccess) {
        [self finish:r mtu:0];
    }
}

- (void)sdpQueryComplete:(IOBluetoothDevice *)device status:(IOReturn)status {
    if (self.closed) {
        return;
    }
    IOBluetoothSDPServiceRecord *rec = [self.device getServiceRecordForUUID:[IOBluetoothSDPUUID uuid16:kBluetoothSDPUUID16ServiceClassSerialPort]];
    if (rec == nil) {
        [self finish:(status != kIOReturnSuccess ? status : -2) mtu:0];
        return;
    }
    [self openChannel:rec];
}

- (void)openChannel:(IOBluetoothSDPServiceRecord *)rec {
    BluetoothRFCOMMChannelID cid = 0;
    IOReturn r = [rec getRFCOMMChannelID:&cid];
    if (r != kIOReturnSuccess) {
        [self finish:r mtu:0];
        return;
    }
    // 用异步打开：手环主动连着 Mac 时，同步接口 openRFCOMMChannelSync 会误报 kIOReturnError
    // （系统日志里通道其实已经连上了），之后的连接全部失败。异步接口等回调就没这个问题。
    IOBluetoothRFCOMMChannel *channel = nil;
    r = [self.device openRFCOMMChannelAsync:&channel withChannelID:cid delegate:self];
    if (r != kIOReturnSuccess || channel == nil) {
        [self finish:(r != kIOReturnSuccess ? r : -3) mtu:0];
        return;
    }
    self.channel = channel;
}

- (void)rfcommChannelOpenComplete:(IOBluetoothRFCOMMChannel *)ch status:(IOReturn)error {
    [self finish:error mtu:(int)[ch getMTU]];
}
- (void)rfcommChannelData:(IOBluetoothRFCOMMChannel *)ch data:(void *)data length:(size_t)length {
    if (!self.closed) {
        goRfcommData(self.handle, data, (int)length);
    }
}
- (void)rfcommChannelClosed:(IOBluetoothRFCOMMChannel *)ch {
    if (!self.closed) {
        goRfcommClosed(self.handle);
    }
}
@end

// 一个进程同时只连一只手环，用全局变量就够了
static MBConn *gConn;

// bt_open 只是把打开连接的第一步派发到主线程，立刻返回；结果由 goRfcommOpened 通知 Go。
void bt_open(const char *address, uintptr_t handle) {
    NSString *addr = [NSString stringWithUTF8String:address];
    dispatch_async(dispatch_get_main_queue(), ^{
        MBConn *conn = [MBConn new];
        conn.handle = handle;
        conn.device = [IOBluetoothDevice deviceWithAddressString:addr];
        gConn = conn;
        if (conn.device == nil) {
            [conn finish:-1 mtu:0];
            return;
        }
        [conn start];
    });
}

int bt_write(const void *data, int length) {
    __block int ret = 0;
    runOnBT(^{
        if (gConn == nil || gConn.channel == nil) {
            ret = -1;
            return;
        }
        int mtu = [gConn.channel getMTU];
        if (mtu <= 0) {
            mtu = 127;
        }
        const uint8_t *p = (const uint8_t *)data;
        for (int off = 0; off < length; off += mtu) {
            int n = length - off < mtu ? length - off : mtu;
            IOReturn r = [gConn.channel writeSync:(void *)(p + off) length:(UInt16)n];
            if (r != kIOReturnSuccess) {
                ret = r;
                return;
            }
        }
    });
    return ret;
}

void bt_close(void) {
    runOnBT(^{
        if (gConn == nil) {
            return;
        }
        gConn.closed = YES;
        if (gConn.channel != nil) {
            [gConn.channel setDelegate:nil];
            [gConn.channel closeChannel];
        }
        if (gConn.device != nil) {
            [gConn.device closeConnection];
        }
        gConn = nil;
    });
}

// ---------- 蓝牙授权 ----------
// 没授权时 IOBluetooth 调用会一直阻塞到用户点完系统弹窗，而我们的调用跑在主线程上，界面就卡死了。
// 所以先用 CoreBluetooth 查授权状态；还没决定时创建 CBCentralManager 触发弹窗（异步，不阻塞）。

@interface MBAuthDelegate : NSObject <CBCentralManagerDelegate>
@end

@implementation MBAuthDelegate
- (void)centralManagerDidUpdateState:(CBCentralManager *)central {
}
@end

static CBCentralManager *gAuthManager;
static MBAuthDelegate *gAuthDelegate;

// 0 未决定，1 受限，2 拒绝，3 允许（CBManagerAuthorization）
int bt_authorization(void) {
    return (int)[CBManager authorization];
}

void bt_request_authorization(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gAuthManager == nil) {
            gAuthDelegate = [MBAuthDelegate new];
            gAuthManager = [[CBCentralManager alloc] initWithDelegate:gAuthDelegate queue:nil];
        }
    });
}
