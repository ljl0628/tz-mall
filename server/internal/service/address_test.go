package service

import (
	"context"
	"testing"

	"fawn-mall/internal/repository"
	"fawn-mall/internal/testutil"
)

func setupAddressSvc(t *testing.T) (*AddressService, *repository.AddressRepo) {
	t.Helper()
	db, _ := testutil.Setup(t)
	repo := repository.NewAddressRepo(db)
	return NewAddressService(repo), repo
}

// clearAddresses 共享测试库,测试前清掉该用户历史地址保证幂等。
func clearAddresses(t *testing.T, repo *repository.AddressRepo, userID uint64) {
	t.Helper()
	list, err := repo.ListByUser(userID)
	if err != nil {
		t.Fatalf("清理前置查询失败: %v", err)
	}
	for _, a := range list {
		if err := repo.Delete(userID, a.ID); err != nil {
			t.Fatalf("清理历史地址失败: %v", err)
		}
	}
}

var addrIn = AddressInput{
	Receiver: "鹿妈妈", Phone: "13800138000",
	Province: "广东省", City: "茂名市", District: "茂南区", Detail: "幸福路 88 号",
}

// TestAddress_FirstAutoDefault 首条地址自动成为默认。
func TestAddress_FirstAutoDefault(t *testing.T) {
	svc, repo := setupAddressSvc(t)
	clearAddresses(t, repo, 101)
	ctx := context.Background()

	a, err := svc.Create(ctx, 101, addrIn)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if !a.IsDefault {
		t.Fatal("首条地址应自动设为默认")
	}
}

// TestAddress_SetDefaultSwitch 默认地址切换后旧默认被清除。
func TestAddress_SetDefaultSwitch(t *testing.T) {
	svc, repo := setupAddressSvc(t)
	clearAddresses(t, repo, 202)
	ctx := context.Background()

	first, err := svc.Create(ctx, 202, addrIn)
	if err != nil {
		t.Fatalf("创建首条失败: %v", err)
	}
	in2 := addrIn
	in2.Detail = "人民路 1 号"
	second, err := svc.Create(ctx, 202, in2)
	if err != nil {
		t.Fatalf("创建第二条失败: %v", err)
	}
	if err := svc.SetDefault(ctx, 202, second.ID); err != nil {
		t.Fatalf("设置默认失败: %v", err)
	}

	list, err := repo.ListByUser(202)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	defaults := 0
	for _, a := range list {
		if a.IsDefault {
			defaults++
			if a.ID != second.ID {
				t.Fatal("默认地址应为第二条")
			}
		}
	}
	if defaults != 1 {
		t.Fatalf("应恰好 1 条默认,实际 %d", defaults)
	}
	_ = first
}

// TestAddress_Ownership 越权操作他人地址被拒。
func TestAddress_Ownership(t *testing.T) {
	svc, repo := setupAddressSvc(t)
	clearAddresses(t, repo, 301)
	ctx := context.Background()

	a, err := svc.Create(ctx, 301, addrIn)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.Update(ctx, 999, a.ID, addrIn); err == nil {
		t.Fatal("他人不应能更新")
	}
	if err := svc.Delete(ctx, 999, a.ID); err == nil {
		t.Fatal("他人不应能删除")
	}
	if err := svc.SetDefault(ctx, 999, a.ID); err == nil {
		t.Fatal("他人不应能设默认")
	}
}

// TestAddress_DeleteDefaultFillsNext 删除默认地址后剩余首条补位。
func TestAddress_DeleteDefaultFillsNext(t *testing.T) {
	svc, repo := setupAddressSvc(t)
	clearAddresses(t, repo, 401)
	ctx := context.Background()

	if _, err := svc.Create(ctx, 401, addrIn); err != nil { // 默认
		t.Fatalf("创建首条失败: %v", err)
	}
	in2 := addrIn
	in2.Detail = "第二街 2 号"
	second, err := svc.Create(ctx, 401, in2)
	if err != nil {
		t.Fatalf("创建第二条失败: %v", err)
	}
	if err := svc.Delete(ctx, 401, second.ID-1); err != nil { // 删掉首条(默认)
		t.Fatalf("删除失败: %v", err)
	}
	list, err := repo.ListByUser(401)
	if err != nil || len(list) != 1 {
		t.Fatalf("应剩 1 条,实际 %d (err=%v)", len(list), err)
	}
	if !list[0].IsDefault {
		t.Fatal("删除默认后剩余地址应补位为默认")
	}
}

// TestAddress_Limit 上限 20 条拦截。
func TestAddress_Limit(t *testing.T) {
	svc, repo := setupAddressSvc(t)
	clearAddresses(t, repo, 501)
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		in := addrIn
		in.Detail = "限量路 "
		if _, err := svc.Create(ctx, 501, in); err != nil {
			t.Fatalf("第 %d 条创建失败: %v", i+1, err)
		}
	}
	if _, err := svc.Create(ctx, 501, addrIn); err == nil {
		t.Fatal("第 21 条应被拒绝")
	}

	t.Cleanup(func() {
		if list, err := repo.ListByUser(501); err == nil {
			for _, a := range list {
				_ = repo.Delete(501, a.ID)
			}
		}
	})
}
