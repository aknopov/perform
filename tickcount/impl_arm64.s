#include "textflag.h"

// ChatGPT solution - not tested!
// This code may fault on "Android kernels or under secure modes"

// func TickCountA() uint64
TEXT ·TickCountA(SB), NOSPLIT, $0-8
    MRS  X0, CNTVCT_EL0       // Read virtual counter into X0
    MOVD X0, ret+0(FP)        // Store into return slot
    RET