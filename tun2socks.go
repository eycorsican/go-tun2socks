package tun2socks

type LWIPStack interface {
	Write([]byte) (int, error)
}
