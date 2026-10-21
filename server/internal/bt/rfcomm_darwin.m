// macOS 经典蓝牙 RFCOMM 的最小封装：打开 SPP 通道、写字节、收字节回调给 Go。

#import <Foundation/Foundation.h>
#import <IOBluetooth/IOBluetooth.h>
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
@property(assign) BOOL openDone;
@property(assign) IOReturn openStatus;
@end

@implementation MBConn
- (void)rfcommChannelOpenComplete:(IOBluetoothRFCOMMChannel *)ch status:(IOReturn)error {
    self.openStatus = error;
    self.openDone = YES;
    goRfcommOpened(self.handle, (int)error, (int)[ch getMTU]);
}
- (void)rfcommChannelData:(IOBluetoothRFCOMMChannel *)ch data:(void *)data length:(size_t)length {
    goRfcommData(self.handle, data, (int)length);
}
- (void)rfcommChannelClosed:(IOBluetoothRFCOMMChannel *)ch {
    goRfcommClosed(self.handle);
}
@end

// 一个进程同时只连一只手环，用全局变量就够了
static MBConn *gConn;

int bt_open(const char *address, uintptr_t handle, int *channelID, int *mtu) {
    __block int ret = 0;
    NSString *addr = [NSString stringWithUTF8String:address];
    runOnBT(^{
        MBConn *conn = [MBConn new];
        conn.handle = handle;
        conn.device = [IOBluetoothDevice deviceWithAddressString:addr];
        if (conn.device == nil) {
            ret = -1;
            return;
        }
        IOReturn r = [conn.device openConnection];
        if (r != kIOReturnSuccess) {
            ret = r;
            return;
        }
        // SDP 查询是异步的，结果靠 run loop 送回来；这里重入 run loop 等它
        IOBluetoothSDPUUID *spp = [IOBluetoothSDPUUID uuid16:kBluetoothSDPUUID16ServiceClassSerialPort];
        IOBluetoothSDPServiceRecord *rec = [conn.device getServiceRecordForUUID:spp];
        if (rec == nil) {
            [conn.device performSDPQuery:nil];
            for (int i = 0; i < 50 && rec == nil; i++) {
                [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.1]];
                rec = [conn.device getServiceRecordForUUID:spp];
            }
        }
        if (rec == nil) {
            ret = -2;
            return;
        }
        BluetoothRFCOMMChannelID cid = 0;
        r = [rec getRFCOMMChannelID:&cid];
        if (r != kIOReturnSuccess) {
            ret = r;
            return;
        }
        // 用异步打开：手环主动连着 Mac 时，同步接口 openRFCOMMChannelSync 会误报 kIOReturnError
        // （系统日志里通道其实已经连上了），之后的连接全部失败。异步接口等回调就没这个问题。
        IOBluetoothRFCOMMChannel *channel = nil;
        r = [conn.device openRFCOMMChannelAsync:&channel withChannelID:cid delegate:conn];
        if (r != kIOReturnSuccess || channel == nil) {
            ret = r != kIOReturnSuccess ? r : -3;
            return;
        }
        // 不能在这里等回调：本块跑在主队列上，回调也走主队列，等就是死锁。结果由 goRfcommOpened 通知 Go。
        conn.channel = channel;
        gConn = conn;
        *channelID = cid;
        *mtu = [channel getMTU];
    });
    return ret;
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
