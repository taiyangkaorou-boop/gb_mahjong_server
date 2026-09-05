package netx

import (
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/logx"
	"github.com/taiyangkaorou-boop/GB_mahjong_server/internal/pb"
	"google.golang.org/protobuf/proto"
)

func Encode(seq uint32, cmd pb.Cmd, code int32, body proto.Message) ([]byte, error) {
	env := &pb.Envelope{Seq: seq, Cmd: cmd, Code: code}
	if body != nil {
		b, err := proto.Marshal(body)
		if err != nil {
			logx.Errorf("netx encode body cmd=%v: %v", cmd, err)
			return nil, err
		}
		env.Body = b
	}
	raw, err := proto.Marshal(env)
	if err != nil {
		logx.Errorf("netx encode cmd=%v: %v", cmd, err)
		return nil, err
	}
	return raw, nil
}

func Decode(raw []byte) (*pb.Envelope, error) {
	env := &pb.Envelope{}
	if err := proto.Unmarshal(raw, env); err != nil {
		logx.Warnf("netx decode: %v", err)
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
