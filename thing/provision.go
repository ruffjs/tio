package thing

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"log/slog"
	"strings"
)

type Provision interface {
	// AutoRegisterViaHmac If the password verification passes, the device is automatically created
	// The password should  comply the generative rule:
	//   echo -n "thingId" | openssl dgst -md5 -hmac "provisionSecret" | xargs -I {} echo H{}
	AutoRegisterViaHmac(ctx context.Context, thingId, password string) (success bool, th Thing, err error)
}

type provisioSvc struct {
	thingSvc   Service
	hmacSecret string
}

func NewProvision(thingSvc Service, hmacSecret string) Provision {
	return &provisioSvc{thingSvc, hmacSecret}
}

func (p *provisioSvc) AutoRegisterViaHmac(ctx context.Context, thingId, password string) (pass bool, th Thing, err error) {
	if !strings.HasPrefix(password, "H") {
		slog.Debug("Provision register failed, password has no H prefix ")
		return false, Thing{}, nil
	}
	hpw := generateHMAC(p.hmacSecret, thingId)
	if password != "H"+hpw {
		slog.Debug("Provision register failed, password mismatch")
		return false, Thing{}, nil
	}
	th, err = p.thingSvc.Create(ctx, Thing{
		Id:        thingId,
		AuthType:  AuthTypePassword,
		AuthValue: password,
		Enabled:   true,
	}, false)
	if err == nil {
		slog.Info("Provision register success", "thingId", thingId)
	} else {
		slog.Info("Provision register failed to create thing ", "thingId", thingId)
	}
	return true, th, err
}

func generateHMAC(key, message string) string {
	h := hmac.New(md5.New, []byte(key))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}
