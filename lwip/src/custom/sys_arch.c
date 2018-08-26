#include "lwip/opt.h"
#include "lwip/sys.h"

#ifdef _WIN64
   //define something for Windows (64-bit)
#elif _WIN32
   //define something for Windows (32-bit)
#elif __APPLE__
    #include "TargetConditionals.h"
    #if TARGET_OS_IPHONE && TARGET_IPHONE_SIMULATOR
        // define something for simulator
    #elif TARGET_OS_IPHONE
        // define something for iphone
    #else
        #define TARGET_OS_OSX 1
        #include <mach/mach_time.h>
        u32_t sys_now(void) {
            uint64_t now = mach_absolute_time();
            mach_timebase_info_data_t info;
            mach_timebase_info(&info);
            now = now * info.numer / info.denom / NSEC_PER_MSEC;
            return (u32_t)(now);
        }
    #endif
#elif __linux
    #include <sys/time.h>
    u32_t sys_now(void)
    {
        struct timeval te;
        gettimeofday(&te, NULL);
        return te.tv_sec*1000LL + te.tv_usec/1000;
    }
#elif __unix // all unices not caught above
    // Unix
#elif __posix
    // POSIX
#endif
