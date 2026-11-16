package service

import (
	"context"
	"errors"

	"sep/common/model"
)

// Launcher 把「啟動一個任務」抽象化，讓 controller 不用知道底下是 K8s 還是 Cloud Run
type Launcher interface {
	// Launch 建立並啟動任務，回傳平台上的 job 名稱
	Launch(ctx context.Context, req model.RunRequest) (jobName string, err error)
}

// ErrRejected 代表請求本身不被允許（系統未註冊、image 不在白名單等），controller 會回 400。
// 其他錯誤一律視為平台端問題，回 500。
var ErrRejected = errors.New("請求被拒絕")
