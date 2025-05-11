#include "textflag.h"

// See https://stackoverflow.com/questions/27039965/how-to-get-the-cpu-cycles-and-executed-time-of-one-function-in-my-source-code-or

TEXT ·TickCountA(SB),7,$0
    RDTSC
    SHLQ  $32, DX
    ADDQ  DX, AX
    MOVQ  AX, ret+0(FP)
    RET

