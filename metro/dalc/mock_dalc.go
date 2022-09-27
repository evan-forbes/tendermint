package dalc

import "sync"

type MockDalc struct {
	mut    *sync.RWMutex
	blocks map[int64]dataSquare
	height int64
}

type dataSquare map[string][][]byte

func (md *MockDalc) GetData(height int64, ns []byte) ([][]byte, bool, error) {
	md.mut.RLock()
	defer md.mut.Unlock()

	d, has := md.blocks[height]
	if !has {
		return nil, false, nil
	}

	b, has := d[string(ns)]
	if !has {
		return nil, false, nil
	}

	return b, true, nil
}

func (md *MockDalc) SubmitData(ns []byte, data []byte, block bool) (int64, error) {
	md.mut.Lock()
	defer md.mut.Unlock()
	h := md.height
	md.blocks[md.height][string(ns)] = append(md.blocks[md.height][string(ns)], data)
	md.height++
	return h, nil
}

func (md *MockDalc) CurrentHeight() (int64, error) {
	md.mut.RLock()
	defer md.mut.Unlock()
	return md.height, nil
}
