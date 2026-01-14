package main

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pb "mesh/internal/meshtastic"
)

func TestNodeInfo(t *testing.T) {
	b, _ := hex.DecodeString("0a092130303030303763311209544d65736820426f741a04544d534822063257000007c12827380842202f20761dd81411ea66c1cf3044fd474612ca92be2e30105178ed6226d74c526c4800")

	v := new(pb.User)

	require.NoError(t, proto.Unmarshal(b, v))
	fmt.Printf("%+v\n", v)

}
