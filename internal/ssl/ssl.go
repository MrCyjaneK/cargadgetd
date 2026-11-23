// Package ssl provides in-band OpenSSL memory-BIO sessions for Android Auto.
package ssl

/*
#cgo LDFLAGS: -lssl -lcrypto -ldl
#include <openssl/ssl.h>
#include <openssl/err.h>
#include <openssl/x509.h>
#include <openssl/pem.h>
#include <openssl/bio.h>
#include <openssl/evp.h>
#include <stdlib.h>

static void cargadgetd_init_openssl(void) {
	OPENSSL_init_ssl(0, NULL);
}

static int cargadgetd_verify_always_ok(int ok, X509_STORE_CTX *ctx) {
	(void)ok;
	(void)ctx;
	return 1;
}

static void cargadgetd_configure_ctx(SSL_CTX *ctx) {
	SSL_CTX_set_min_proto_version(ctx, TLS1_2_VERSION);
	SSL_CTX_set_max_proto_version(ctx, TLS1_2_VERSION);
	SSL_CTX_set_verify(ctx, SSL_VERIFY_PEER, cargadgetd_verify_always_ok);
	SSL_CTX_set_dh_auto(ctx, 1);
}

static int cargadgetd_use_cert_pem(SSL_CTX *ctx, const char *pem, int len) {
	BIO *bio = BIO_new_mem_buf((void *)pem, len);
	if (!bio) {
		return 0;
	}
	X509 *cert = PEM_read_bio_X509(bio, NULL, NULL, NULL);
	BIO_free(bio);
	if (!cert) {
		return 0;
	}
	int ok = SSL_CTX_use_certificate(ctx, cert);
	X509_free(cert);
	return ok;
}

static int cargadgetd_use_key_pem(SSL_CTX *ctx, const char *pem, int len) {
	BIO *bio = BIO_new_mem_buf((void *)pem, len);
	if (!bio) {
		return 0;
	}
	EVP_PKEY *key = PEM_read_bio_PrivateKey(bio, NULL, NULL, NULL);
	BIO_free(bio);
	if (!key) {
		return 0;
	}
	int ok = SSL_CTX_use_PrivateKey(ctx, key);
	EVP_PKEY_free(key);
	return ok;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"unsafe"
)

func init() {
	C.cargadgetd_init_openssl()
}

// Session is an MD-side (TLS server) or HU-side (TLS client) in-band SSL session.
type Session struct {
	mu     sync.Mutex
	ctx    *C.SSL_CTX
	ssl    *C.SSL
	server bool
}

func newBlankCtx() (*C.SSL_CTX, error) {
	ctx := C.SSL_CTX_new(C.TLS_method())
	if ctx == nil {
		return nil, errors.New("ssl: SSL_CTX_new failed")
	}
	C.cargadgetd_configure_ctx(ctx)
	return ctx, nil
}

func newCtx(certFile, keyFile string) (*C.SSL_CTX, error) {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("ssl: read cert %s: %w", certFile, err)
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("ssl: read key %s: %w", keyFile, err)
	}
	return newCtxPEM(certPEM, keyPEM)
}

func newCtxPEM(certPEM, keyPEM []byte) (*C.SSL_CTX, error) {
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return nil, errors.New("ssl: empty cert or key PEM")
	}
	ctx, err := newBlankCtx()
	if err != nil {
		return nil, err
	}
	if C.cargadgetd_use_cert_pem(ctx, (*C.char)(unsafe.Pointer(&certPEM[0])), C.int(len(certPEM))) <= 0 {
		C.SSL_CTX_free(ctx)
		return nil, errors.New("ssl: load cert PEM failed")
	}
	if C.cargadgetd_use_key_pem(ctx, (*C.char)(unsafe.Pointer(&keyPEM[0])), C.int(len(keyPEM))) <= 0 {
		C.SSL_CTX_free(ctx)
		return nil, errors.New("ssl: load key PEM failed")
	}
	return ctx, nil
}

func newSession(ctx *C.SSL_CTX, server bool) (*Session, error) {
	s := &Session{ctx: ctx, server: server}
	if err := s.resetSSL(); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// NewServer creates an MD TLS-server session from PEM files on disk.
func NewServer(certFile, keyFile string) (*Session, error) {
	ctx, err := newCtx(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return newSession(ctx, true)
}

// NewServerPEM creates an MD TLS-server session from in-memory PEM bytes.
func NewServerPEM(certPEM, keyPEM []byte) (*Session, error) {
	ctx, err := newCtxPEM(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return newSession(ctx, true)
}

// NewClient creates an HU TLS-client session from PEM files on disk.
func NewClient(certFile, keyFile string) (*Session, error) {
	ctx, err := newCtx(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return newSession(ctx, false)
}

// NewClientPEM creates an HU TLS-client session from in-memory PEM bytes.
func NewClientPEM(certPEM, keyPEM []byte) (*Session, error) {
	ctx, err := newCtxPEM(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return newSession(ctx, false)
}

func (s *Session) resetSSL() error {
	s.ssl = C.SSL_new(s.ctx)
	if s.ssl == nil {
		return errors.New("ssl: SSL_new failed")
	}
	rb := C.BIO_new(C.BIO_s_mem())
	wb := C.BIO_new(C.BIO_s_mem())
	if rb == nil || wb == nil {
		return errors.New("ssl: BIO_new failed")
	}
	C.SSL_set_bio(s.ssl, rb, wb)
	if s.server {
		C.SSL_set_accept_state(s.ssl)
	} else {
		C.SSL_set_connect_state(s.ssl)
	}
	return nil
}

func (s *Session) writeBIO() *C.BIO {
	// SSL_get_wbio
	return C.SSL_get_wbio(s.ssl)
}

func (s *Session) readBIO() *C.BIO {
	return C.SSL_get_rbio(s.ssl)
}

// FeedHandshake writes peer TLS bytes and drives accept/connect.
func (s *Session) FeedHandshake(peerTLS []byte) (out []byte, done bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(peerTLS) > 0 {
		n := C.BIO_write(s.readBIO(), unsafe.Pointer(&peerTLS[0]), C.int(len(peerTLS)))
		if int(n) <= 0 {
			return nil, false, errors.New("ssl: BIO_write failed")
		}
	}
	var ret C.int
	if s.server {
		ret = C.SSL_accept(s.ssl)
	} else {
		ret = C.SSL_connect(s.ssl)
	}
	out = s.drainWriteBIO()
	if ret == 1 {
		return out, true, nil
	}
	errN := C.SSL_get_error(s.ssl, ret)
	if errN == C.SSL_ERROR_WANT_READ || errN == C.SSL_ERROR_WANT_WRITE {
		return out, false, nil
	}
	return out, false, fmt.Errorf("ssl: handshake error %d", int(errN))
}

// Encrypt returns TLS ciphertext for plain application data.
// Callers must keep len(plain) ≤ one TLS record (≤16384); larger buffers make
// OpenSSL emit multiple records, and AA/DHU typically SSL_read once per AA frame.
func (s *Session) Encrypt(plain []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ssl == nil {
		// Close() races the session writer draining its queues on teardown;
		// SSL_write(NULL) is a segfault, not an error.
		return nil, errors.New("ssl: session closed")
	}
	if len(plain) == 0 {
		return nil, nil
	}
	var out []byte
	wrote := 0
	for len(plain) > 0 {
		n := C.SSL_write(s.ssl, unsafe.Pointer(&plain[0]), C.int(len(plain)))
		if int(n) <= 0 {
			errN := C.SSL_get_error(s.ssl, n)
			if errN == C.SSL_ERROR_WANT_READ || errN == C.SSL_ERROR_WANT_WRITE {
				out = append(out, s.drainWriteBIO()...)
				continue
			}
			return nil, fmt.Errorf("ssl: SSL_write failed (%d)", int(errN))
		}
		out = append(out, s.drainWriteBIO()...)
		wrote += int(n)
		plain = plain[int(n):]
	}
	if nrec := countTLSAppRecords(out); nrec != 1 {
		// AA/DHU SSL_read once per AA frame - multi-record ciphertext desyncs the peer.
		return nil, fmt.Errorf("ssl: Encrypt plain=%d produced %d TLS records (need 1)", wrote, nrec)
	}
	return out, nil
}

func countTLSAppRecords(ct []byte) int {
	n, off := 0, 0
	for off+5 <= len(ct) {
		l := int(ct[off+3])<<8 | int(ct[off+4])
		if l < 0 || off+5+l > len(ct) {
			return -1
		}
		n++
		off += 5 + l
	}
	if off != len(ct) {
		return -1
	}
	return n
}

// Decrypt consumes ciphertext and returns plaintext.
func (s *Session) Decrypt(cipher []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ssl == nil {
		return nil, errors.New("ssl: session closed")
	}
	if len(cipher) == 0 {
		return nil, nil
	}
	if C.BIO_write(s.readBIO(), unsafe.Pointer(&cipher[0]), C.int(len(cipher))) <= 0 {
		return nil, errors.New("ssl: BIO_write (decrypt) failed")
	}
	buf := make([]byte, 100*1024)
	n := C.SSL_read(s.ssl, unsafe.Pointer(&buf[0]), C.int(len(buf)))
	if int(n) <= 0 {
		errN := C.SSL_get_error(s.ssl, n)
		if errN == C.SSL_ERROR_WANT_READ || errN == C.SSL_ERROR_WANT_WRITE {
			return nil, nil
		}
		return nil, fmt.Errorf("ssl: SSL_read failed (%d)", int(errN))
	}
	return buf[:n], nil
}

func (s *Session) drainWriteBIO() []byte {
	var out []byte
	buf := make([]byte, 4096)
	wb := s.writeBIO()
	for {
		n := C.BIO_read(wb, unsafe.Pointer(&buf[0]), C.int(len(buf)))
		if int(n) <= 0 {
			break
		}
		out = append(out, buf[:n]...)
	}
	return out
}

// Close frees OpenSSL resources.
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ssl != nil {
		C.SSL_free(s.ssl)
		s.ssl = nil
	}
	if s.ctx != nil {
		C.SSL_CTX_free(s.ctx)
		s.ctx = nil
	}
}
