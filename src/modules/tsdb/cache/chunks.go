package cache

import (
	"fmt"
	"sync"

	"github.com/toolkits/pkg/logger"
)

type CS struct {
	Chunks          []*Chunk
	CurrentChunkPos int // 记录当切片 chunk块的位置 数量
	flag            uint32

	sync.RWMutex
}

func NewChunks(numOfChunks int) *CS {
	cs := make([]*Chunk, 0, numOfChunks)

	return &CS{Chunks: cs}
}

func (cs *CS) Push(seriesID string, ts int64, value float64) error {
	//找到当前chunk的起始时间
	t0 := uint32(ts - (ts % int64(Config.SpanInSeconds))) // 按照时间间隔统计，t0为这一时间区间的起始时间

	// 尚无chunk
	if len(cs.Chunks) == 0 { // 每个cache 有很多块里面 存切片 按照时间段统计 数据
		c := NewChunk(uint32(t0))
		c.FirstTs = uint32(ts)
		cs.Chunks = append(cs.Chunks, c)

		return cs.Chunks[0].Push(uint32(ts), value) // 统计这个区间的次数+1
	}

	// push到当前chunk
	currentChunk := cs.GetChunk(cs.CurrentChunkPos)
	if t0 == currentChunk.T0 { // 时间处于最新区间
		if currentChunk.Closed { // 当前块没有关闭，才进行统计次数+1
			return fmt.Errorf("push to closed chunk")
		}

		return currentChunk.Push(uint32(ts), value) // 统计+1
	}

	if t0 < currentChunk.T0 { // 传输数据时间落后于当前区间，不统计本次
		return fmt.Errorf("data @%v, timestamp old than previous chunk. currentchunk t0: %v\n", t0, currentChunk.T0)
	}

	// 需要新建chunk
	// 先finish掉现有chunk
	if !currentChunk.Closed { // 否则，数据块超前，新建数据块，关闭本块数据
		currentChunk.FinishSync()
		ChunksSlots.Push(seriesID, currentChunk)
	}

	// 超过chunks限制, pos回绕到0
	cs.CurrentChunkPos++
	if cs.CurrentChunkPos >= int(Config.NumOfChunks) { // 块下标数量，如果大于最大块设置的数量，初始化从0开始
		cs.CurrentChunkPos = 0
	}

	// chunks未满, 直接append即可
	if len(cs.Chunks) < int(Config.NumOfChunks) { // 未达到块最大数量。直接新建，追加
		c := NewChunk(uint32(t0))
		c.FirstTs = uint32(ts)
		cs.Chunks = append(cs.Chunks, c)

		return cs.Chunks[cs.CurrentChunkPos].Push(uint32(ts), value)
	} else { // 否则新建，替换[0]旧的块是数据
		c := NewChunk(uint32(t0))
		c.FirstTs = uint32(ts)
		cs.Chunks[cs.CurrentChunkPos] = c // 替换切片[0]开始的位置，从头开始存储块数据

		return cs.Chunks[cs.CurrentChunkPos].Push(uint32(ts), value)
	}

	return nil
}

func (cs *CS) Get(from, to int64) []Iter {
	// 这种case不应该发生
	if from >= to { // 起始时间范围判断
		return nil
	}

	cs.RLock()
	defer cs.RUnlock()

	// cache server还没有数据
	if len(cs.Chunks) == 0 {
		return nil
	}

	var iters []Iter

	// from 超出最新chunk可能达到的最新点, 这种case不应该发生
	newestChunk := cs.GetChunk(cs.CurrentChunkPos)
	if from >= int64(newestChunk.T0)+int64(Config.SpanInSeconds) {
		return nil
	}

	// 假设共有2个chunk
	// len = 1, CurrentChunkPos = 0, oldestPos = 0
	// len = 2, CurrentChunkPos = 0, oldestPos = 1
	// len = 2, CurrentChunkPos = 1, oldestPos = 0
	oldestPos := cs.CurrentChunkPos + 1
	if oldestPos >= len(cs.Chunks) {
		oldestPos = 0
	}
	oldestChunk := cs.GetChunk(oldestPos)
	if oldestChunk == nil {
		logger.Error("unexpected nil chunk")
		return nil
	}

	// to 太老了, 这种case不应发生, 应由query处理
	if to <= int64(oldestChunk.FirstTs) {
		return nil
	}

	// 找from所在的chunk
	for from >= int64(oldestChunk.T0)+int64(Config.SpanInSeconds) {
		oldestPos++
		if oldestPos >= len(cs.Chunks) {
			oldestPos = 0
		}
		oldestChunk = cs.GetChunk(oldestPos)
		if oldestChunk == nil {
			logger.Error("unexpected nil chunk")
			return nil
		}
	}

	// 找to所在的trunk
	newestPos := cs.CurrentChunkPos
	for to <= int64(newestChunk.T0) {
		newestPos--
		if newestPos < 0 {
			newestPos += len(cs.Chunks)
		}
		newestChunk = cs.GetChunk(newestPos)
		if newestChunk == nil {
			logger.Error("unexpected nil chunk")
			return nil
		}
	}

	for {
		c := cs.GetChunk(oldestPos)
		iters = append(iters, NewIter(c.Iter()))
		if oldestPos == newestPos {
			break
		}
		oldestPos++
		if oldestPos >= len(cs.Chunks) {
			oldestPos = 0
		}
	}

	return iters
}

// GetInfo get oldest ts and newest ts in cache
func (cs *CS) GetInfo() (uint32, uint32) {
	cs.RLock()
	defer cs.RUnlock()

	return cs.GetInfoUnsafe()
}

func (cs *CS) GetInfoUnsafe() (uint32, uint32) {
	var oldestTs, newestTs uint32

	if len(cs.Chunks) == 0 {
		return 0, 0
	}

	newestChunk := cs.GetChunk(cs.CurrentChunkPos)
	if newestChunk == nil {
		newestTs = 0
	} else {
		newestTs = newestChunk.LastTs
	}

	oldestPos := cs.CurrentChunkPos + 1
	if oldestPos >= len(cs.Chunks) {
		oldestPos = 0
	}

	oldestChunk := cs.GetChunk(oldestPos)
	if oldestChunk == nil {
		oldestTs = 0
	} else {
		oldestTs = oldestChunk.FirstTs
	}

	return oldestTs, newestTs
}

func (cs *CS) GetFlag() uint32 {
	cs.RLock()
	defer cs.RUnlock()

	return cs.flag
}

func (cs *CS) SetFlag(flag uint32) {
	cs.Lock()
	defer cs.Unlock()

	cs.flag = flag
	return
}

func (cs CS) GetChunk(pos int) *Chunk {
	if pos < 0 || pos >= len(cs.Chunks) {
		return cs.Chunks[0]
	}

	return cs.Chunks[pos]
}
