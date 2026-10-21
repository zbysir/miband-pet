#include <stdint.h>

int bt_open(const char *address, uintptr_t handle, int *channelID, int *mtu);
int bt_write(const void *data, int length);
void bt_close(void);
void bt_run_main_loop(void);
