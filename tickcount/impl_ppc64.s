#include "textflag.h"

// Not tested!

TEXT ·TickCountA(SB),7,$0
		MFTBU R3
		MFTBL R4
		ORIS R3,R3,0
		SLWI R3,R3,32
		OR R3,R3,R4
		STW R3, 0(5)
		RET
