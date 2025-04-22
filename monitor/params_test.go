package monitor

import (
	"flag"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParamTypeCoverage(t *testing.T) {
	assertT := assert.New(t)

	// collect all enum values
	allParams := make([]ParamType, 0)
	for p := paramLast; p >= paramFirst; p-- {
		allParams = append(allParams, p)
	}

	namedParams := make([]ParamType, 0)
	for p, _ := range nameMap {
		namedParams = append(namedParams, p)
	}
	assertT.ElementsMatch(allParams, namedParams)

	convertParams := make([]ParamType, 0)
	for _, p := range convertMap {
		convertParams = append(convertParams, p)
	}
	assertT.ElementsMatch(allParams, convertParams)
}

func TestParseParamList(t *testing.T) {
	assertT := assert.New(t)

	var paramList ParamList
	assertT.Nil(parseParamList("Cpu,Mem", &paramList))
	assertT.ElementsMatch([]ParamType{Cpu, Mem}, paramList)

	paramList = paramList[0:0]
	assertT.Nil(parseParamList("Cpu,Mem,PIDs,CPUs,Rx,Tx", &paramList))
	assertT.ElementsMatch([]ParamType{Cpu, Mem, PIDs, CPUs, Rx, Tx}, paramList)

	paramList = paramList[0:0]
	assertT.Nil(parseParamList("Cpu, Mem", &paramList))
	assertT.ElementsMatch([]ParamType{Cpu, Mem}, paramList)

	paramList = paramList[0:0]
	assertT.Nil(parseParamList("Cpu,Mem, Foo", &paramList))
	assertT.ElementsMatch([]ParamType{Cpu, Mem}, paramList)
}

// Google AI generate
func TestParseParams(t *testing.T) {
	assertT := assert.New(t)

	testCases := []struct {
		name       string
		args       []string
		expName    string
		expIntvl   float32
		expParms   ParamList
		shouldFail bool
	}{
		{
			name:       "Normal",
			args:       []string{"-params=", "Cpu, Mem", "ID"},
			expName:    "ID",
			expIntvl:   1.0,
			expParms:   []ParamType{Cpu, Mem},
			shouldFail: false,
		},
		{
			name:       "Normal",
			args:       []string{"-params=", "Cpu, Mem", "-refresh=", "10", "ID"},
			expName:    "ID",
			expIntvl:   1.0,
			expParms:   []ParamType{Cpu, Mem},
			shouldFail: false,
		},
		{
			name:       "No ID",
			args:       []string{"-params=", "CpuPerc, MemPerc"},
			expName:    "",
			expIntvl:   1.0,
			expParms:   []ParamType{},
			shouldFail: true,
		},
	}

	for _, tc := range testCases {
		flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
		name, params, intvl, err := ParseParams(flagSet, tc.args)

		if !tc.shouldFail {
			assertT.NoError(err, "In test", tc.name)
			return
		}

		assertT.Equal(tc.expName, name, "In test", tc.name)
		assertT.Equal(tc.expIntvl, intvl, "In test", tc.name)
		assertT.ElementsMatch(tc.expParms, params, "In test", tc.name)
	}
}

func TestPrintHeader(t *testing.T) {
	assertT := assert.New(t)

	stream, ch := CreateStream()

	var paramList ParamList = ParamList{CPUs, Tx}
	PrintHeader(stream, &paramList)

	output := ReadStream(stream, ch)
	assertT.Equal("Time                              CPUs    Tx MBps\n", output)

}

func TestPrintValues(t *testing.T) {
	assertT := assert.New(t)

	stream, ch := CreateStream()

	PrintValues(stream, []float32{1.0, 13.0})

	output := ReadStream(stream, ch)
	tsRex := regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3} .*`)
	assertT.True(tsRex.MatchString(output))
	assertT.True(strings.HasSuffix(output, "         1.00       13.00\n"))
}
