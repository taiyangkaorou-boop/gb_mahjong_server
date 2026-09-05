package netx

import (
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/pb"
	"google.golang.org/protobuf/proto"
)

func Encode(seq uint32, cmd pb.Cmd, code int32, body proto.Message) ([]byte, error) {
	env := &pb.Envelope{Seq: seq, Cmd: cmd, Code: code}
	if body != nil {
		b, err := proto.Marshal(body)
		if err != nil {
			return nil, err
		}
		env.Body = b
	}
	return proto.Marshal(env)
}

func Decode(raw []byte) (*pb.Envelope, error) {
	env := &pb.Envelope{}
	if err := proto.Unmarshal(raw, env); err != nil {
		return nil, err
	}
	return env, nil
}

func UnmarshalBody(env *pb.Envelope, msg proto.Message) error {
	if len(env.Body) == 0 {
		return nil
	}
	return proto.Unmarshal(env.Body, msg)
}
