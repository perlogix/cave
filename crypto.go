package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"

	"github.com/denisbrodbeck/machineid"
)

//Crypto struct
type Crypto struct {
	sharedkey *AESKey
	privkey   *AESKey
	id        string
}

//AESKey type
type AESKey struct {
	Key       []byte
	NonceSize int
	Pass      []byte
}

func (a *AESKey) newGCM() (cipher.AEAD, error) {
	block, err := aes.NewCipher(a.Pass)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func newCrypto() (*Crypto, error) {
	id, err := machineid.ID()
	if err != nil {
		return nil, err
	}
	c := &Crypto{
		id: id,
	}
	err = c.GenerateSystemAES()
	if err != nil {
		return c, err
	}
	return c, nil
}

func (c *Crypto) createHash(key string) string {
	hasher := md5.New()
	_, err := hasher.Write([]byte(key))
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

// GenerateSharedKey function
func (c *Crypto) GenerateSharedKey() error {
	pass := make([]byte, 32) // new random 12-byte passphrase
	if _, err := io.ReadFull(rand.Reader, pass); err != nil {
		return err
	}
	block, err := aes.NewCipher(pass)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	c.sharedkey = &AESKey{
		Key:       nonce,
		NonceSize: gcm.NonceSize(),
		Pass:      pass,
	}
	return nil
}

// GenerateSystemAES function
func (c *Crypto) GenerateSystemAES() error {
	id, err := machineid.ID()
	if err != nil {
		return err
	}
	block, err := aes.NewCipher([]byte(c.createHash(id)))
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	buf := bytes.NewBuffer([]byte(c.createHash(id)[:gcm.NonceSize()]))
	if _, err = io.ReadFull(buf, nonce); err != nil {
		return err
	}
	c.privkey = &AESKey{
		Key:       nonce,
		NonceSize: gcm.NonceSize(),
		Pass:      []byte(c.createHash(id)),
	}
	return nil
}

//SealSharedKey function
func (c *Crypto) SealSharedKey(sharedkey *AESKey, privkey *AESKey, overwrite bool) error {
	if _, err := os.Stat("cluster.key"); !os.IsNotExist(err) && !overwrite {
		// dont overwrite key if it exists
		return nil
	}
	n := privkey.Key
	f, err := os.OpenFile("cluster.key", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	bkey, err := json.Marshal(sharedkey)
	if err != nil {
		return err
	}
	gcm, err := privkey.newGCM()
	if err != nil {
		return err
	}
	txt := gcm.Seal(n, privkey.Key, bkey, nil)
	_, err = f.Write(txt)
	if err != nil {
		return err
	}
	return nil
}

//UnsealSharedKey function
func (c *Crypto) UnsealSharedKey(privkey *AESKey) (*AESKey, error) {
	var b []byte
	f, err := ioutil.ReadFile("cluster.key")
	if err != nil {
		return nil, err
	}
	gcm, err := privkey.newGCM()
	if err != nil {
		return nil, err
	}
	n, text := f[:privkey.NonceSize], f[privkey.NonceSize:]
	b, err = gcm.Open(nil, n, text, nil)
	if err != nil {
		return nil, err
	}
	var sharedkey *AESKey
	err = json.Unmarshal(b, &sharedkey)
	if err != nil {
		return nil, err
	}
	return sharedkey, nil
}

func encrytJSON(key *AESKey, data interface{}) ([]byte, error) {
	var secret []byte
	n := key.Key
	b, err := json.Marshal(data)
	if err != nil {
		return secret, err
	}
	gcm, err := key.newGCM()
	if err != nil {
		return secret, err
	}
	txt := gcm.Seal(n, key.Key, b, nil)
	secret = append([]byte("SECRET+"), txt...)
	return secret, nil
}

func decryptJSON(key *AESKey, data []byte) ([]byte, error) {
	var secret []byte
	gcm, err := key.newGCM()
	if err != nil {
		return nil, err
	}
	sec := data[len([]byte("SECRET+")):]
	n, text := sec[:key.NonceSize], sec[key.NonceSize:]
	s, err := gcm.Open(nil, n, text, nil)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(s, &secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
}
