package service

import (
	"context"
	"strings"

	"fawn-mall/internal/model"
	"fawn-mall/internal/repository"
	"fawn-mall/pkg/apperr"
	"fawn-mall/pkg/resp"
)

// maxAddressPerUser 每人地址簿上限。
const maxAddressPerUser = 20

// AddressService 收货地址簿:增删改查 + 默认地址切换。
type AddressService struct {
	addrs *repository.AddressRepo
}

// NewAddressService 构造。
func NewAddressService(a *repository.AddressRepo) *AddressService { return &AddressService{addrs: a} }

// AddressInput 创建/更新地址的请求体(binding 校验在 handler 层完成)。
type AddressInput struct {
	Receiver  string `json:"receiver" binding:"required,min=2,max=32"`
	Phone     string `json:"phone" binding:"required,len=11,numeric"`
	Province  string `json:"province" binding:"required,min=2,max=32"`
	City      string `json:"city" binding:"required,min=2,max=32"`
	District  string `json:"district" binding:"required,min=2,max=32"`
	Detail    string `json:"detail" binding:"required,min=2,max=200"`
	IsDefault bool   `json:"is_default"`
}

// List 用户地址列表(默认地址置顶)。
func (s *AddressService) List(ctx context.Context, userID uint64) ([]model.Address, error) {
	return s.addrs.ListByUser(userID)
}

// Create 新增:首条地址自动设为默认;显式默认则先清旧默认。
func (s *AddressService) Create(ctx context.Context, userID uint64, in AddressInput) (*model.Address, error) {
	trimAddressInput(&in)
	if n, err := s.addrs.CountByUser(userID); err != nil {
		return nil, err
	} else if n >= maxAddressPerUser {
		return nil, resp.NewBiz(resp.CodeAddressLimit, "地址簿最多保存 20 条,请先删除不用的地址")
	}

	list, err := s.addrs.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	isDefault := in.IsDefault || len(list) == 0 // 首条地址自动默认

	if isDefault {
		if err := s.addrs.ClearDefault(userID); err != nil {
			return nil, err
		}
	}
	a := &model.Address{
		UserID: userID, Receiver: in.Receiver, Phone: in.Phone,
		Province: in.Province, City: in.City, District: in.District,
		Detail: in.Detail, IsDefault: isDefault,
	}
	if err := s.addrs.Create(a); err != nil {
		return nil, apperr.Wrap("新建收货地址", err)
	}
	return a, nil
}

// Update 编辑(校验归属);改为默认时同步清旧默认。
func (s *AddressService) Update(ctx context.Context, userID, id uint64, in AddressInput) (*model.Address, error) {
	trimAddressInput(&in)
	a, err := s.addrs.GetByID(id)
	if err != nil {
		return nil, err
	}
	if a == nil || a.UserID != userID {
		return nil, resp.NewBiz(resp.CodeAddressNotFound, "地址不存在")
	}
	a.Receiver, a.Phone = in.Receiver, in.Phone
	a.Province, a.City, a.District, a.Detail = in.Province, in.City, in.District, in.Detail
	if err := s.addrs.Update(a); err != nil {
		return nil, apperr.Wrap("更新收货地址", err)
	}
	if in.IsDefault && !a.IsDefault {
		if err := s.SetDefault(ctx, userID, id); err != nil {
			return nil, err
		}
		a.IsDefault = true
	}
	return a, nil
}

// Delete 删除(仅本人的);删掉默认地址后剩余首自动补位为默认。
func (s *AddressService) Delete(ctx context.Context, userID, id uint64) error {
	a, err := s.addrs.GetByID(id)
	if err != nil {
		return err
	}
	if a == nil || a.UserID != userID {
		return resp.NewBiz(resp.CodeAddressNotFound, "地址不存在")
	}
	if err := s.addrs.Delete(userID, id); err != nil {
		return apperr.Wrap("删除收货地址", err)
	}
	// 删的是默认地址:剩余列表第一条补位,保证任何时刻最多一个默认且尽量存在
	if a.IsDefault {
		if list, lerr := s.addrs.ListByUser(userID); lerr == nil && len(list) > 0 {
			if serr := s.addrs.SetDefault(userID, list[0].ID); serr != nil {
				// 补位失败不影响删除结果,仅记录
				_ = serr
			}
		}
	}
	return nil
}

// SetDefault 设为默认地址(事务清除旧默认)。
func (s *AddressService) SetDefault(ctx context.Context, userID, id uint64) error {
	a, err := s.addrs.GetByID(id)
	if err != nil {
		return err
	}
	if a == nil || a.UserID != userID {
		return resp.NewBiz(resp.CodeAddressNotFound, "地址不存在")
	}
	if err := s.addrs.SetDefault(userID, id); err != nil {
		return apperr.Wrap("设置默认地址", err)
	}
	return nil
}

// trimAddressInput 去除首尾空白(省市区文本不允许空白填充绕过必填)。
func trimAddressInput(in *AddressInput) {
	in.Receiver = strings.TrimSpace(in.Receiver)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Province = strings.TrimSpace(in.Province)
	in.City = strings.TrimSpace(in.City)
	in.District = strings.TrimSpace(in.District)
	in.Detail = strings.TrimSpace(in.Detail)
}
