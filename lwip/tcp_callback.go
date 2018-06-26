package lwip

/*
#cgo CFLAGS: -I./src/include
#include "lwip/tcp.h"

extern err_t TCPAcceptFn(void *arg, struct tcp_pcb *newpcb, err_t err);

void
set_tcp_accept_callback(struct tcp_pcb *pcb) {
	tcp_accept(pcb, TCPAcceptFn);
}

extern err_t TCPRecvFn(void *arg, struct tcp_pcb *tpcb, struct pbuf *p, err_t err);

void
set_tcp_recv_callback(struct tcp_pcb *pcb) {
	tcp_recv(pcb, TCPRecvFn);
}

extern err_t TCPSentFn(void *arg, struct tcp_pcb *tpcb, u16_t len);

void
set_tcp_sent_callback(struct tcp_pcb *pcb) {
    tcp_sent(pcb, TCPSentFn);
}

extern void TCPErrFn(void *arg, err_t err);

void
set_tcp_err_callback(struct tcp_pcb *pcb) {
	tcp_err(pcb, TCPErrFn);
}

extern err_t TCPPollFn(void *arg, struct tcp_pcb *tpcb);

void
set_tcp_poll_callback(struct tcp_pcb *pcb, u8_t interval) {
	tcp_poll(pcb, TCPPollFn, interval);
}
*/
import "C"

func SetTCPAcceptCallback(pcb *C.struct_tcp_pcb) {
	C.set_tcp_accept_callback(pcb)
}

func SetTCPRecvCallback(pcb *C.struct_tcp_pcb) {
	C.set_tcp_recv_callback(pcb)
}

func SetTCPSentCallback(pcb *C.struct_tcp_pcb) {
	C.set_tcp_sent_callback(pcb)
}

func SetTCPErrCallback(pcb *C.struct_tcp_pcb) {
	C.set_tcp_err_callback(pcb)
}

func SetTCPPollCallback(pcb *C.struct_tcp_pcb, interval C.u8_t) {
	C.set_tcp_poll_callback(pcb, interval)
}
