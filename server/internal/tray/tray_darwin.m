// 菜单栏图标（NSStatusItem）：关掉窗口后 App 在这里常驻。AppKit 调用都派发到主线程。

#import <Cocoa/Cocoa.h>
#include "tray_darwin.h"
#include "_cgo_export.h"

@interface MBTrayTarget : NSObject
@end

@implementation MBTrayTarget
- (void)show:(id)sender {
    goTrayShow();
}
- (void)quit:(id)sender {
    goTrayQuit();
}
@end

static NSStatusItem *gItem;
static NSMenuItem *gInfo;
static MBTrayTarget *gTarget;

void tray_init(const void *png, int length) {
    NSData *data = [NSData dataWithBytes:png length:length];
    dispatch_async(dispatch_get_main_queue(), ^{
        gTarget = [MBTrayTarget new];
        gItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
        NSImage *image = [[NSImage alloc] initWithData:data];
        image.size = NSMakeSize(18, 18); // 图片是 @2x 的 36px
        gItem.button.image = image;
        gItem.button.toolTip = @"miband pet";

        NSMenu *menu = [NSMenu new];
        gInfo = [[NSMenuItem alloc] initWithTitle:@"正在连接手环…" action:nil keyEquivalent:@""];
        gInfo.enabled = NO;
        [menu addItem:gInfo];
        [menu addItem:[NSMenuItem separatorItem]];
        NSMenuItem *show = [[NSMenuItem alloc] initWithTitle:@"打开主窗口" action:@selector(show:) keyEquivalent:@""];
        show.target = gTarget;
        [menu addItem:show];
        [menu addItem:[NSMenuItem separatorItem]];
        NSMenuItem *quit = [[NSMenuItem alloc] initWithTitle:@"退出 miband pet" action:@selector(quit:) keyEquivalent:@"q"];
        quit.target = gTarget;
        [menu addItem:quit];
        gItem.menu = menu;
    });
}

void tray_set_info(const char *text) {
    NSString *s = [NSString stringWithUTF8String:text];
    dispatch_async(dispatch_get_main_queue(), ^{
        gInfo.title = s;
    });
}

// 窗口隐藏时连 Dock 图标一起隐藏，只留菜单栏图标
void tray_set_dock_visible(int visible) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (visible) {
            [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
            [NSApp activateIgnoringOtherApps:YES];
        } else {
            [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
        }
    });
}
