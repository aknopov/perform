package net

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/aknopov/perform/mocker"
	"github.com/google/gopacket/pcap"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	tIdx        = 0
	addr1       = net.Addr{IP: "127.0.0.1", Port: 12345}
	addr2       = net.Addr{IP: "127.0.0.1", Port: 12346}
	addr3       = net.Addr{IP: "127.0.0.1", Port: 12347}
	connections = [][]net.ConnectionStat{
		{net.ConnectionStat{Laddr: addr1, Pid: 222}},
		{net.ConnectionStat{Laddr: addr1, Pid: 111}, net.ConnectionStat{Laddr: addr2, Pid: 111}},
		{net.ConnectionStat{Laddr: addr1, Pid: 111}, net.ConnectionStat{Laddr: addr3, Pid: 111}},
		{net.ConnectionStat{Laddr: addr1, Pid: 111}, net.ConnectionStat{Laddr: addr3, Pid: 111}},
	}
	expire = 20 * time.Millisecond
)

func mockConnsProvider(_ context.Context, _ string) ([]net.ConnectionStat, error) {
	ret := connections[tIdx]
	tIdx++
	return ret, nil
}

func TestSingleStartInvocation(t *testing.T) {
	defer mocker.ReplaceItem(&procConnMap, nil)()
	defer mocker.ReplaceItem(&findAllDevs, func() ([]pcap.Interface, error) { return []pcap.Interface{}, nil })

	var errChan chan error

	ctx, cancel := context.WithCancel(context.Background())
	assert.NotPanics(t, func() { errChan = StartTracing(ctx, 1, time.Millisecond) })
	cancel()
	assert.Eventually(t, func() bool { <-ctx.Done(); return true }, time.Second, 10*time.Millisecond)

	assert.Panics(t, func() { errChan = StartTracing(context.Background(), 1, time.Millisecond) })

	select {
	case err := <-errChan:
		rex := regexp.MustCompile("You don't have permission to.*capture on that device")
		if !rex.MatchString(err.Error()) {
			t.Fatalf("No error expected - got %v", err)
		}
	case <-time.After(50 * time.Millisecond):
	}
}

func TestGetProcConnMapCopy(t *testing.T) {
	defer mocker.ReplaceItem(&tIdx, 0)()
	defer mocker.ReplaceItem(&procConnMap, make(map[net.Addr]*procNetStat))()

	updateTable(context.Background(), 111, mockConnsProvider, expire)
	updateTable(context.Background(), 111, mockConnsProvider, expire)

	procNetStatCpy := getProcConnMapCopy()
	assert.NotEmpty(t, procNetStatCpy)

	// check deep copy
	for k, v := range procConnMap {
		vCopy := procNetStatCpy[k]
		assert.NotEqual(t, v.netCounters, &vCopy.netCounters)
	}
}

func TestGetProcessNetIOCounters(t *testing.T) {
	testTab := make(map[net.Addr]*procNetStat)
	stat1 := IOCountersStat{BytesSent: 111, BytesRecv: 222, PacketsSent: 3, PacketsRecv: 4}
	stat2 := IOCountersStat{BytesSent: 333, BytesRecv: 44, PacketsSent: 2, PacketsRecv: 2}
	stat3 := IOCountersStat{BytesSent: 444, BytesRecv: 57, PacketsSent: 5, PacketsRecv: 7}
	testTab[net.Addr{IP: "192.168.0.235", Port: 20781}] = createNetStat(111, stat1)
	testTab[net.Addr{IP: "127.0.0.1", Port: 22137}] = createNetStat(111, stat2)
	testTab[net.Addr{IP: "192.168.0.235", Port: 20675}] = createNetStat(411, stat3)
	defer mocker.ReplaceItem(&procConnMap, testTab)()
	defer mocker.ReplaceItem(&pid, 111)()

	counts, err := GetProcessNetIOCounters()
	require.NoError(t, err)
	assert.EqualValues(t, 444, counts.BytesSent)
	assert.EqualValues(t, 266, counts.BytesRecv)
	assert.EqualValues(t, 5, counts.PacketsSent)
	assert.EqualValues(t, 6, counts.PacketsRecv)
	assert.Zero(t, counts.Errin)
	assert.Zero(t, counts.Errout)

	defer mocker.ReplaceItem(&pid, 777)()
	_, err = GetProcessNetIOCounters()
	assert.Error(t, err)
}

func TestConnMapRefresh(t *testing.T) {
	defer mocker.ReplaceItem(&tIdx, 0)()
	defer mocker.ReplaceItem(&procConnMap, make(map[net.Addr]*procNetStat))()

	assert.Empty(t, procConnMap)

	updateTable(context.Background(), 111, mockConnsProvider, expire)
	assert.Len(t, procConnMap, 0)

	updateTable(context.Background(), 111, mockConnsProvider, expire)
	assert.Len(t, procConnMap, 2)
	assert.Contains(t, procConnMap, addr1)
	assert.Contains(t, procConnMap, addr2)
	assert.NotContains(t, procConnMap, addr3)

	updateTable(context.Background(), 111, mockConnsProvider, expire)
	assert.Len(t, procConnMap, 3)
	assert.Contains(t, procConnMap, addr1)
	assert.Contains(t, procConnMap, addr2)
	assert.Contains(t, procConnMap, addr3)

	time.Sleep(2 * expire)
	updateTable(context.Background(), 111, mockConnsProvider, expire)
	assert.Len(t, procConnMap, 3)
	assert.Contains(t, procConnMap, addr1)
	assert.Contains(t, procConnMap, addr2)
	assert.Contains(t, procConnMap, addr3)
}

func TestHandlingErrorsOnRefresh(t *testing.T) {
	testErrChan := make(chan error)
	defer mocker.ReplaceItem(&errChan, testErrChan)()

	mockConnProvider := func(_ context.Context, _ string) ([]net.ConnectionStat, error) { return nil, errors.New("test") }

	go updateTable(context.Background(), -1, mockConnProvider, time.Second)

	assert.Error(t, <-testErrChan)
}

func BenchmarkUpdateTable(b *testing.B) {
	defer mocker.ReplaceItem(&procConnMap, make(map[net.Addr]*procNetStat))()

	b.ResetTimer()

	for range b.N {
		updateTable(context.Background(), -1, net.ConnectionsWithContext, time.Second)
	}
}

func BenchmarkGetProcConnMapCopy(b *testing.B) {
	defer mocker.ReplaceItem(&procConnMap, make(map[net.Addr]*procNetStat))()

	b.ResetTimer()

	for range b.N {
		getProcConnMapCopy()
	}
}
