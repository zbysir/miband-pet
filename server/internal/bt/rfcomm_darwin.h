#include <stdint.h>

void bt_open(const char *address, uintptr_t handle);
int bt_write(const void *data, int length);
void bt_close(void);
void bt_run_main_loop(void);
int bt_authorization(void);
void bt_request_authorization(void);
