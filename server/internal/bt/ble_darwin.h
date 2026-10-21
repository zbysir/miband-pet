#include <stdint.h>

void ble_scan_start(const char *macReversed, int macLen, const char *nameSuffix, uintptr_t handle);
void ble_scan_stop(void);
void ble_connect(const char *identifier, uintptr_t handle);
void ble_write(const void *data, int length, int chunk);
void ble_disconnect(void);
