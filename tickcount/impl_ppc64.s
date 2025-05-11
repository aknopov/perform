#include "textflag.h"

// ChatGPT solution - not tested!

// func TickCountA() uint64
TEXT ·TickCountA(SB), NOSPLIT, $0-8
Loop:
    MFSPR R4, 269     // TBU -> R4
    MFSPR R5, 268     // TBL -> R5
    MFSPR R6, 269     // TBU again -> R6
    CMP   R4, R6
    BNE   Loop        // Retry if TBU changed

    RLDIMI R4, R5, 32, 0  // Insert R5 into low 32 bits of R4
    MOVD   R4, ret+0(FP)  // Store result to return value
    RET

/* Alt
TEXT ·TickCountA(SB),7,$0
		MFTBU R3
		MFTBL R4
		ORIS R3,R3,0
		SLWI R3,R3,32
		OR R3,R3,R4
		STW R3, 0(5)
		BLR
*/