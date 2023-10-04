package ctrl

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"golang.org/x/crypto/ssh"
)

// nkeyOptFromSSHKey will read the ED25519 SSH private key file,
// then convert the content into a NKEY Seed file, and return
// the result as a nats.Option.
func (c *Configuration) nkeyOptFromSSHKey() (nats.Option, error) {
	// --- Create key
	p, err := c.CreateKeyPair(PrefixByteUser)
	if err != nil {
		return nil, fmt.Errorf("error: createpair failed: %v", err)
	}

	filepathSeed := filepath.Join(c.SocketFolder, "seed_from_ssh_key.txt")
	filepathUser := filepath.Join(c.SocketFolder, "user_from_ssh_key.txt")

	// Write the private key seed to file.
	{
		fh, err := os.OpenFile(filepathSeed, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0660)
		if err != nil {
			return nil, fmt.Errorf("error: open file for writing seed failed: %v", err)
		}
		defer fh.Close()
		_, err = fh.Write(p.seed)
		if err != nil {
			return nil, fmt.Errorf("error: writing of  seed file for ssh key failed: %v", err)
		}
	}
	// Write the user key to file.
	{
		fh, err := os.OpenFile(filepathUser, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0660)
		if err != nil {
			return nil, fmt.Errorf("error: open file for writing user failed: %v", err)
		}
		defer fh.Close()
		_, err = fh.Write([]byte(p.userKey))
		if err != nil {
			return nil, fmt.Errorf("error: writing of user file for ssh key failed: %v", err)
		}
	}

	opt, err := nats.NkeyOptionFromSeed(filepathSeed)
	if err != nil {
		return nil, fmt.Errorf("error: creating option nkey from seed file failed: %v", err)
	}

	return opt, nil

}

// Convert the ed25519 public key to nkey user key.
func publicKeyToUserKey(signer ssh.Signer) (string, error) {
	key := signer.PublicKey()

	marshalled := key.Marshal()
	seed := marshalled[len(marshalled)-32:]

	encoded, err := nkeys.Encode(nkeys.PrefixByteUser, seed)
	if err != nil {
		return "", err
	}

	return string(encoded), nil
}

type PrefixByte byte

// PrefixByteUser is the version byte used for encoded NATS Users
const PrefixByteUser PrefixByte = 20 << 3 // Base32-encodes to 'U...'

type kp struct {
	seed    []byte
	userKey string
}

// CreateKeyPair will create the key nkeys key pair.
// This is a slightly modified version of the function
// found in the nkeys package, just with less options.
func (c *Configuration) CreateKeyPair(prefix PrefixByte) (kp, error) {
	fh, err := os.Open(c.NkeyFromED25519SSHKeyFile)
	if err != nil {
		return kp{}, fmt.Errorf("failed: open ssh private key file: %v", err)
	}
	defer fh.Close()

	b, err := io.ReadAll(fh)
	if err != nil {
		return kp{}, fmt.Errorf("failed: read ssh private key file: %v", err)
	}

	signerRaw, err := ssh.ParseRawPrivateKey(b)
	if err != nil {
		return kp{}, fmt.Errorf("failed: parse raw private key: %v", err)
	}

	switch signerRaw.(type) {
	case *ed25519.PrivateKey:
		// ok, got correct type, just continue.
	default:
		t := reflect.TypeOf(signerRaw)
		return kp{}, fmt.Errorf("failed: key is wrong type, want type ed25519 key, got: %v", t)
	}
	// ---
	raw := signerRaw.(*ed25519.PrivateKey)
	bytePrivAndPub := []byte(*raw)

	seed, err := EncodeSeed(prefix, bytePrivAndPub[:32])
	if err != nil {
		return kp{}, err
	}

	signer, err := ssh.ParsePrivateKey(b)
	if err != nil {
		return kp{}, fmt.Errorf("failed: parse private key: %v", err)
	}

	pub, err := publicKeyToUserKey(signer)
	if err != nil {
		return kp{}, fmt.Errorf("failed: failed to get public key for signer: %v", err)
	}

	return kp{seed: seed, userKey: pub}, nil
}

// PrefixByteSeed is the version byte used for encoded NATS Seeds
const PrefixByteSeed PrefixByte = 18 << 3 // Base32-encodes to 'S...'

// EncodeSeed will encode a raw key with the prefix and then seed prefix and crc16 and then base32 encoded.
// This is a slightly modified version of the function
// found in the nkeys package, just with less options.
func EncodeSeed(public PrefixByte, src []byte) ([]byte, error) {

	if len(src) != ed25519.SeedSize {
		return nil, fmt.Errorf("invalid key length")
	}

	// In order to make this human printable for both bytes, we need to do a little
	// bit manipulation to setup for base32 encoding which takes 5 bits at a time.
	b1 := byte(PrefixByteSeed) | (byte(public) >> 5)
	b2 := (byte(public) & 31) << 3 // 31 = 00011111

	var raw bytes.Buffer

	raw.WriteByte(b1)
	raw.WriteByte(b2)

	// write payload
	if _, err := raw.Write(src); err != nil {
		return nil, err
	}

	// Calculate and write crc16 checksum
	err := binary.Write(&raw, binary.LittleEndian, crc16(raw.Bytes()))
	if err != nil {
		return nil, err
	}

	data := raw.Bytes()
	buf := make([]byte, b32Enc.EncodedLen(len(data)))
	b32Enc.Encode(buf, data)
	return buf, nil
}

// Set our encoding to not include padding '=='
var b32Enc = base32.StdEncoding.WithPadding(base32.NoPadding)

// crc16 returns the 2-byte crc for the data provided.
// This is a slightly modified version of the function
// found in the nkeys package, just with less options.
func crc16(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc = ((crc << 8) & 0xffff) ^ crc16tab[((crc>>8)^uint16(b))&0x00FF]
	}
	return crc
}

var crc16tab = [256]uint16{
	0x0000, 0x1021, 0x2042, 0x3063, 0x4084, 0x50a5, 0x60c6, 0x70e7,
	0x8108, 0x9129, 0xa14a, 0xb16b, 0xc18c, 0xd1ad, 0xe1ce, 0xf1ef,
	0x1231, 0x0210, 0x3273, 0x2252, 0x52b5, 0x4294, 0x72f7, 0x62d6,
	0x9339, 0x8318, 0xb37b, 0xa35a, 0xd3bd, 0xc39c, 0xf3ff, 0xe3de,
	0x2462, 0x3443, 0x0420, 0x1401, 0x64e6, 0x74c7, 0x44a4, 0x5485,
	0xa56a, 0xb54b, 0x8528, 0x9509, 0xe5ee, 0xf5cf, 0xc5ac, 0xd58d,
	0x3653, 0x2672, 0x1611, 0x0630, 0x76d7, 0x66f6, 0x5695, 0x46b4,
	0xb75b, 0xa77a, 0x9719, 0x8738, 0xf7df, 0xe7fe, 0xd79d, 0xc7bc,
	0x48c4, 0x58e5, 0x6886, 0x78a7, 0x0840, 0x1861, 0x2802, 0x3823,
	0xc9cc, 0xd9ed, 0xe98e, 0xf9af, 0x8948, 0x9969, 0xa90a, 0xb92b,
	0x5af5, 0x4ad4, 0x7ab7, 0x6a96, 0x1a71, 0x0a50, 0x3a33, 0x2a12,
	0xdbfd, 0xcbdc, 0xfbbf, 0xeb9e, 0x9b79, 0x8b58, 0xbb3b, 0xab1a,
	0x6ca6, 0x7c87, 0x4ce4, 0x5cc5, 0x2c22, 0x3c03, 0x0c60, 0x1c41,
	0xedae, 0xfd8f, 0xcdec, 0xddcd, 0xad2a, 0xbd0b, 0x8d68, 0x9d49,
	0x7e97, 0x6eb6, 0x5ed5, 0x4ef4, 0x3e13, 0x2e32, 0x1e51, 0x0e70,
	0xff9f, 0xefbe, 0xdfdd, 0xcffc, 0xbf1b, 0xaf3a, 0x9f59, 0x8f78,
	0x9188, 0x81a9, 0xb1ca, 0xa1eb, 0xd10c, 0xc12d, 0xf14e, 0xe16f,
	0x1080, 0x00a1, 0x30c2, 0x20e3, 0x5004, 0x4025, 0x7046, 0x6067,
	0x83b9, 0x9398, 0xa3fb, 0xb3da, 0xc33d, 0xd31c, 0xe37f, 0xf35e,
	0x02b1, 0x1290, 0x22f3, 0x32d2, 0x4235, 0x5214, 0x6277, 0x7256,
	0xb5ea, 0xa5cb, 0x95a8, 0x8589, 0xf56e, 0xe54f, 0xd52c, 0xc50d,
	0x34e2, 0x24c3, 0x14a0, 0x0481, 0x7466, 0x6447, 0x5424, 0x4405,
	0xa7db, 0xb7fa, 0x8799, 0x97b8, 0xe75f, 0xf77e, 0xc71d, 0xd73c,
	0x26d3, 0x36f2, 0x0691, 0x16b0, 0x6657, 0x7676, 0x4615, 0x5634,
	0xd94c, 0xc96d, 0xf90e, 0xe92f, 0x99c8, 0x89e9, 0xb98a, 0xa9ab,
	0x5844, 0x4865, 0x7806, 0x6827, 0x18c0, 0x08e1, 0x3882, 0x28a3,
	0xcb7d, 0xdb5c, 0xeb3f, 0xfb1e, 0x8bf9, 0x9bd8, 0xabbb, 0xbb9a,
	0x4a75, 0x5a54, 0x6a37, 0x7a16, 0x0af1, 0x1ad0, 0x2ab3, 0x3a92,
	0xfd2e, 0xed0f, 0xdd6c, 0xcd4d, 0xbdaa, 0xad8b, 0x9de8, 0x8dc9,
	0x7c26, 0x6c07, 0x5c64, 0x4c45, 0x3ca2, 0x2c83, 0x1ce0, 0x0cc1,
	0xef1f, 0xff3e, 0xcf5d, 0xdf7c, 0xaf9b, 0xbfba, 0x8fd9, 0x9ff8,
	0x6e17, 0x7e36, 0x4e55, 0x5e74, 0x2e93, 0x3eb2, 0x0ed1, 0x1ef0,
}
