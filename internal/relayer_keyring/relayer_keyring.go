package relayer_keyring

import (
	"bytes"
	"fmt"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	neutronapp "github.com/neutron-org/neutron/app"
	"io"
)

type passReader struct {
	pass string
	buf  *bytes.Buffer
}

func newPassReader(pass string) io.Reader {
	return &passReader{
		pass: pass,
		buf:  new(bytes.Buffer),
	}
}

func (r *passReader) Read(p []byte) (n int, err error) {
	n, err = r.buf.Read(p)
	if err == io.EOF || n == 0 {
		r.buf.WriteString(r.pass + "\n")

		n, err = r.buf.Read(p)
	}

	return n, err
}

func InitializeKeyring(keyringBackend, keyringPassword, homeDir, keyName, keySeed, hdPath string) (keyring.Keyring, string, error) {
	passReader := newPassReader(keyringPassword)

	keybase, err := keyring.New(neutronapp.Bech32MainPrefix, keyringBackend, homeDir, passReader)
	if err != nil {
		return nil, "", fmt.Errorf("error creating keybase of type %s and keyringRootDir=%s: %w", keyringBackend, homeDir, err)
	}

	// If the keybase is set to "memory" then we expect the seed to be passed via environment variable and we need to
	// add the key to the in-memory keybase
	if keyringBackend == keyring.BackendMemory {
		// For in-memory key we ignore the name provided by user (so it might be (and actually should be) left empty)
		keyName = "sign_key"
		if len(hdPath) == 0 {
			hdPath = hd.CreateHDPath(sdk.CoinType, 0, 0).String()
		}
		_, err := keybase.NewAccount(keyName, keySeed, "", hdPath, hd.Secp256k1)
		if err != nil {
			return nil, "", err
		}
	}
	return keybase, keyName, nil
}
