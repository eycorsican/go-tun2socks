#include "lwip/opt.h"
#include "lwip/sys.h"

#if DARWIN
#include <mach/mach_time.h>

u32_t sys_now(void) {
    uint64_t now = mach_absolute_time();
    mach_timebase_info_data_t info;
    mach_timebase_info(&info);
    now = now * info.numer / info.denom / NSEC_PER_MSEC;
    
    return (u32_t)(now);
}
#elif LINUX
#include <sys/time.h>

u32_t sys_now(void)
{
    struct timeval te; 
    gettimeofday(&te, NULL);
    return te.tv_sec*1000LL + te.tv_usec/1000;
}
#endif
