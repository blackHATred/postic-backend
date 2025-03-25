package entity

type Upload struct {
	RawBytes []byte
	FileName string
	FileType string
	UserID   int
}
