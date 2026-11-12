package service

// 管理端用户管理测试(真实 MySQL + Redis):
// 列表筛选 / 封禁与解封的即时生效链路 / 角色调整覆盖 / 重置密码 / 自我保护规则。

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"fawn-mall/internal/model"
	"fawn-mall/internal/repository"
	"fawn-mall/internal/testutil"
	"fawn-mall/pkg/jwtx"
	"fawn-mall/pkg/resp"

	"gorm.io/gorm"
)

type adminUserEnv struct {
	admin  *AdminService
	auth   *AuthService
	ustate *repository.UserState
	db     *gorm.DB
	opID   uint64 // 操作的管理员 ID
}

func newAdminUserEnv(t *testing.T) *adminUserEnv {
	t.Helper()
	db, rdb := testutil.Setup(t)
	users := repository.NewUserRepo(db)
	black := repository.NewBlacklist(rdb)
	jwt := jwtx.New("test-secret", 1)
	ustate := repository.NewUserState(rdb, time.Hour)
	env := &adminUserEnv{
		admin:  NewAdminService(nil, nil, nil, nil, nil, users, ustate),
		auth:   NewAuthService(users, black, jwt),
		ustate: ustate,
		db:     db,
	}
	// 独立的管理员操作者账号
	op, err := env.auth.Register(context.Background(), RegisterReq{
		Username: fmt.Sprintf("admop_%d", time.Now().UnixNano()),
		Password: "admin123456",
	})
	if err != nil {
		t.Fatalf("创建管理员操作者失败: %v", err)
	}
	if err := db.Model(&model.User{}).Where("id = ?", op.User.ID).Update("role", model.RoleAdmin).Error; err != nil {
		t.Fatalf("提升操作者为管理员失败: %v", err)
	}
	env.opID = op.User.ID
	t.Cleanup(func() { db.Delete(&model.User{}, op.User.ID) })
	return env
}

// newUser 注册一个临时普通用户并注册清理。
func (e *adminUserEnv) newUser(t *testing.T, suffix string) *model.User {
	t.Helper()
	out, err := e.auth.Register(context.Background(), RegisterReq{
		Username: fmt.Sprintf("u_%s_%d", suffix, time.Now().UnixNano()),
		Password: "pass123456",
	})
	if err != nil {
		t.Fatalf("注册测试用户失败: %v", err)
	}
	t.Cleanup(func() {
		e.db.Delete(&model.User{}, out.User.ID)
		e.ustate.ClearDisabled(context.Background(), out.User.ID)
		e.ustate.ClearRole(context.Background(), out.User.ID)
	})
	return &out.User
}

func TestAdminListUsersFilter(t *testing.T) {
	e := newAdminUserEnv(t)
	ctx := context.Background()
	u1 := e.newUser(t, "flt1")

	page, err := e.admin.AdminListUsers(ctx, repository.UserFilter{Keyword: u1.Username})
	if err != nil {
		t.Fatalf("列表查询失败: %v", err)
	}
	if page.Total != 1 || len(page.List) != 1 || page.List[0].ID != u1.ID {
		t.Fatalf("关键词筛选应恰好命中新建用户: total=%d list=%v", page.Total, page.List)
	}

	// 角色筛选:新用户是 user
	page, err = e.admin.AdminListUsers(ctx, repository.UserFilter{Keyword: u1.Username, Role: model.RoleAdmin})
	if err != nil {
		t.Fatalf("列表查询失败: %v", err)
	}
	if page.Total != 0 {
		t.Fatalf("角色筛选 admin 不应命中普通用户, got total=%d", page.Total)
	}

	// 状态筛选:封禁后按 status=0 能查到
	if err := e.admin.SetUserStatus(ctx, e.opID, u1.ID, model.UserStatusDisabled); err != nil {
		t.Fatalf("封禁失败: %v", err)
	}
	st := int8(model.UserStatusDisabled)
	page, err = e.admin.AdminListUsers(ctx, repository.UserFilter{Keyword: u1.Username, Status: &st})
	if err != nil {
		t.Fatalf("列表查询失败: %v", err)
	}
	if page.Total != 1 {
		t.Fatalf("状态筛选 disabled 应命中 1 人, got %d", page.Total)
	}
}

func TestSetUserStatusDisableAndEnable(t *testing.T) {
	e := newAdminUserEnv(t)
	ctx := context.Background()
	u := e.newUser(t, "dis")

	// 封禁:MySQL 状态 + Redis 覆盖键 + 登录被拒
	if err := e.admin.SetUserStatus(ctx, e.opID, u.ID, model.UserStatusDisabled); err != nil {
		t.Fatalf("封禁失败: %v", err)
	}
	if disabled, _ := e.ustate.IsDisabled(ctx, u.ID); !disabled {
		t.Fatal("封禁后 Redis 覆盖键应存在")
	}
	_, err := e.auth.Login(ctx, LoginReq{Username: u.Username, Password: "pass123456"})
	if !errors.Is(err, resp.ErrUserDisabled) {
		t.Fatalf("封禁后登录应返回 ErrUserDisabled, got %v", err)
	}

	// 解封:状态恢复、覆盖键清除、登录恢复
	if err := e.admin.SetUserStatus(ctx, e.opID, u.ID, model.UserStatusActive); err != nil {
		t.Fatalf("解封失败: %v", err)
	}
	if disabled, _ := e.ustate.IsDisabled(ctx, u.ID); disabled {
		t.Fatal("解封后 Redis 覆盖键应清除")
	}
	if _, err := e.auth.Login(ctx, LoginReq{Username: u.Username, Password: "pass123456"}); err != nil {
		t.Fatalf("解封后登录应成功, got %v", err)
	}
}

func TestSetUserRole(t *testing.T) {
	e := newAdminUserEnv(t)
	ctx := context.Background()
	u := e.newUser(t, "role")

	// 升级为管理员:DB 角色 + Redis 角色覆盖 + 重新登录拿到 admin 角色
	if err := e.admin.SetUserRole(ctx, e.opID, u.ID, model.RoleAdmin); err != nil {
		t.Fatalf("升级管理员失败: %v", err)
	}
	if role, _ := e.ustate.RoleOverride(ctx, u.ID); role != model.RoleAdmin {
		t.Fatalf("角色覆盖键应为 admin, got %q", role)
	}
	out, err := e.auth.Login(ctx, LoginReq{Username: u.Username, Password: "pass123456"})
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if out.User.Role != model.RoleAdmin {
		t.Fatalf("升级后登录角色应为 admin, got %s", out.User.Role)
	}

	// 幂等:重复设置同角色不报错
	if err := e.admin.SetUserRole(ctx, e.opID, u.ID, model.RoleAdmin); err != nil {
		t.Fatalf("重复设置同角色应幂等成功, got %v", err)
	}
}

func TestResetUserPassword(t *testing.T) {
	e := newAdminUserEnv(t)
	ctx := context.Background()
	u := e.newUser(t, "pwd")

	if err := e.admin.ResetUserPassword(ctx, u.ID, "newpass654321"); err != nil {
		t.Fatalf("重置密码失败: %v", err)
	}
	// 旧密码失效
	if _, err := e.auth.Login(ctx, LoginReq{Username: u.Username, Password: "pass123456"}); !errors.Is(err, resp.ErrLoginFailed) {
		t.Fatalf("旧密码登录应失败, got %v", err)
	}
	// 新密码可用
	if _, err := e.auth.Login(ctx, LoginReq{Username: u.Username, Password: "newpass654321"}); err != nil {
		t.Fatalf("新密码登录应成功, got %v", err)
	}
}

func TestUserSelfProtection(t *testing.T) {
	e := newAdminUserEnv(t)
	ctx := context.Background()

	if err := e.admin.SetUserStatus(ctx, e.opID, e.opID, model.UserStatusDisabled); !errors.Is(err, resp.ErrSelfOp) {
		t.Fatalf("封禁自己应返回 ErrSelfOp, got %v", err)
	}
	if err := e.admin.SetUserRole(ctx, e.opID, e.opID, model.RoleUser); !errors.Is(err, resp.ErrSelfOp) {
		t.Fatalf("修改自己角色应返回 ErrSelfOp, got %v", err)
	}
	// 不存在的用户
	if err := e.admin.SetUserStatus(ctx, e.opID, 99999999, model.UserStatusDisabled); !errors.Is(err, resp.ErrUserNotFound) {
		t.Fatalf("操作不存在用户应返回 ErrUserNotFound, got %v", err)
	}
}
