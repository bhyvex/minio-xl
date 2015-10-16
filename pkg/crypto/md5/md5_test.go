package md5_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/minio/minio-xl/pkg/crypto/md5"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type MySuite struct{}

var _ = Suite(&MySuite{})

func (s *MySuite) TestMd5sum(c *C) {
	testString := []byte("Test string")
	expectedHash, _ := hex.DecodeString("0fd3dbec9730101bff92acc820befc34")
	hash, err := md5.Sum(bytes.NewBuffer(testString))
	c.Assert(err, IsNil)
	c.Assert(bytes.Equal(expectedHash, hash), Equals, true)
}
