package dalc

type DALC interface {
	SubmitData(ns []byte, data []byte) (height, err error)
	GetData(height int64, ns []byte) ([][]byte, bool, error)
	CurrentHeight() (int64, error)
}
