// Package seed 种子数据:分类/商品/SKU/账号。各表为空时才写入,幂等可重复执行。
package seed

import (
	"fmt"

	"fawn-mall/internal/model"
	"fawn-mall/internal/repository"
	"fawn-mall/pkg/logger"
	"fawn-mall/pkg/password"
	"gorm.io/gorm"
)

// 测试账号(README 中公开)。
const (
	AdminUsername = "admin"
	AdminPassword = "admin123"
	DemoUsername  = "demo"
	DemoPassword  = "demo123"
)

type seedSKU struct {
	Size  string
	Stock int
}

type seedProduct struct {
	Category    string // 分类名(按名查 ID)
	Name        string
	AgeGroup    string
	Material    string
	Season      string
	Description string
	Price       int64 // 分
	Recommended int8
	Sizes       []seedSKU
}

var categories = []model.Category{
	{Name: "连体衣", Sort: 1},
	{Name: "套装", Sort: 2},
	{Name: "上衣", Sort: 3},
	{Name: "裤装", Sort: 4},
	{Name: "裙装", Sort: 5},
	{Name: "外套", Sort: 6},
	{Name: "配饰", Sort: 7},
}

var products = []seedProduct{
	{Name: "婴儿纯棉和尚领连体衣", Category: "连体衣", AgeGroup: model.AgeBaby, Material: "100% 精梳棉", Season: model.SeasonSpringSummer,
		Description: "和尚领设计不勒脖,天然彩棉柔软透气,裆部按扣方便换尿布,适合 0-3 岁婴幼儿贴身穿著。",
		Price:       6900, Recommended: 1, Sizes: []seedSKU{{"59", 50}, {"66", 60}, {"73", 45}}},
	{Name: "婴儿夹棉保暖连体衣", Category: "连体衣", AgeGroup: model.AgeBaby, Material: "棉布+聚酯纤维夹棉", Season: model.SeasonAutumnWinter,
		Description: "三层夹棉锁温,连体包裹不漏风,腿部双头拉链,夜间换尿布不冻肚子。",
		Price:       12900, Recommended: 1, Sizes: []seedSKU{{"66", 40}, {"73", 40}, {"80", 35}, {"90", 30}}},
	{Name: "新生儿透气短袖爬服", Category: "连体衣", AgeGroup: model.AgeBaby, Material: "60S 莫代尔棉", Season: model.SeasonSpringSummer,
		Description: "轻薄如羽,吸湿排汗速干,适合夏季新生儿贴身穿着,袖口防刮设计。",
		Price:       5900, Sizes: []seedSKU{{"52", 45}, {"59", 50}, {"66", 50}}},
	{Name: "宝宝菠萝纹连体爬服", Category: "连体衣", AgeGroup: model.AgeBaby, Material: "棉+氨纶罗纹", Season: model.SeasonAutumnWinter,
		Description: "弹性罗纹面料随宝宝成长伸展,包裹感强,学爬期膝盖加厚耐磨。",
		Price:       7900, Sizes: []seedSKU{{"73", 35}, {"80", 35}, {"90", 30}}},

	{Name: "宝宝背带裤两件套", Category: "套装", AgeGroup: model.AgeBaby, Material: "水洗棉", Season: model.SeasonSpringSummer,
		Description: "上衣+背带裤一套搞定,背带四档可调,穿到两岁不浪费。",
		Price:       9900, Recommended: 1, Sizes: []seedSKU{{"80", 40}, {"90", 40}, {"100", 30}}},
	{Name: "小童学院风三件套", Category: "套装", AgeGroup: model.AgeLittle, Material: "棉+聚酯纤维", Season: model.SeasonAutumnWinter,
		Description: "马甲+衬衫+长裤,入园拍照体面之选,面料挺括不易皱。",
		Price:       19900, Recommended: 1, Sizes: []seedSKU{{"100", 30}, {"110", 30}, {"120", 25}}},
	{Name: "小童运动休闲两件套", Category: "套装", AgeGroup: model.AgeLittle, Material: "针织棉", Season: model.SeasonSpringSummer,
		Description: "T恤+短裤,透气网眼,跑跳不受限,幼儿园活动常备。",
		Price:       13900, Sizes: []seedSKU{{"100", 35}, {"110", 35}, {"120", 30}}},
	{Name: "中童速干运动套装", Category: "套装", AgeGroup: model.AgeMiddle, Material: "速干聚酯纤维", Season: model.SeasonSpringSummer,
		Description: "速干面料,大童户外运动、亲子跑步皆宜,反光条夜跑更安全。",
		Price:       16900, Sizes: []seedSKU{{"130", 25}, {"140", 25}, {"150", 25}, {"160", 20}}},

	{Name: "小童圆领条纹短袖T恤", Category: "上衣", AgeGroup: model.AgeLittle, Material: "精梳棉", Season: model.SeasonSpringSummer,
		Description: "经典海军条纹,领口双针加固不变形,耐洗不褪色。",
		Price:       4900, Recommended: 1, Sizes: []seedSKU{{"100", 60}, {"110", 60}, {"120", 50}, {"130", 40}}},
	{Name: "中童印花短袖T恤", Category: "上衣", AgeGroup: model.AgeMiddle, Material: "莫代尔混纺", Season: model.SeasonSpringSummer,
		Description: "莫代尔手感丝滑,卡通印花采用环保水性油墨,安心贴身。",
		Price:       5900, Sizes: []seedSKU{{"130", 45}, {"140", 45}, {"150", 40}, {"160", 35}}},
	{Name: "大童落肩袖摇粒绒卫衣", Category: "上衣", AgeGroup: model.AgeBig, Material: "摇粒绒", Season: model.SeasonAutumnWinter,
		Description: "宽松落肩剪裁,内里摇粒绒温暖轻量,校园日常百搭款。",
		Price:       11900, Recommended: 1, Sizes: []seedSKU{{"150", 30}, {"160", 30}, {"165", 25}}},
	{Name: "婴儿包屁衣短袖哈衣", Category: "上衣", AgeGroup: model.AgeBaby, Material: "有机棉", Season: model.SeasonSpringSummer,
		Description: "有机棉认证,包臀设计防止衣服上蹿,三档按扣好穿脱。",
		Price:       4500, Sizes: []seedSKU{{"59", 55}, {"66", 55}, {"73", 45}}},

	{Name: "小童灯芯绒背带裤", Category: "裤装", AgeGroup: model.AgeLittle, Material: "灯芯绒(棉)", Season: model.SeasonAutumnWinter,
		Description: "复古灯芯绒,11 条绒纹理清晰,膝盖处耐磨加固。",
		Price:       10900, Sizes: []seedSKU{{"100", 35}, {"110", 35}, {"120", 30}}},
	{Name: "中童束脚运动长裤", Category: "裤装", AgeGroup: model.AgeMiddle, Material: "针织棉+氨纶", Season: model.SeasonSpringSummer,
		Description: "松紧腰+抽绳双保险,束脚利落,体育课首选。",
		Price:       7900, Sizes: []seedSKU{{"130", 40}, {"140", 40}, {"150", 35}, {"160", 30}}},
	{Name: "大童直筒弹力牛仔裤", Category: "裤装", AgeGroup: model.AgeBig, Material: "弹力牛仔布(棉)", Season: model.SeasonAutumnWinter,
		Description: "弹力牛仔不紧绷,直筒版型显腿长,腰部调节扣适配腰围。",
		Price:       13900, Recommended: 1, Sizes: []seedSKU{{"150", 30}, {"160", 30}, {"165", 25}}},
	{Name: "婴儿加厚夹棉长裤", Category: "裤装", AgeGroup: model.AgeBaby, Material: "棉+聚酯纤维夹棉", Season: model.SeasonAutumnWinter,
		Description: "高腰护肚,夹棉温暖,裤脚按扣可开合,换尿布神器。",
		Price:       6900, Sizes: []seedSKU{{"73", 40}, {"80", 40}, {"90", 35}}},

	{Name: "婴儿网纱公主纱裙", Category: "裙装", AgeGroup: model.AgeBaby, Material: "网纱+棉内衬", Season: model.SeasonSpringSummer,
		Description: "双层网纱蓬蓬裙摆,内衬纯棉不扎皮肤,百日宴拍照首选。",
		Price:       8900, Recommended: 1, Sizes: []seedSKU{{"80", 35}, {"90", 35}, {"100", 30}}},
	{Name: "小童碎花雪纺连衣裙", Category: "裙装", AgeGroup: model.AgeLittle, Material: "雪纺(聚酯纤维)", Season: model.SeasonSpringSummer,
		Description: "清新碎花印花,雪纺轻盈飘逸,内衬防透,夏日出游首选。",
		Price:       9900, Recommended: 1, Sizes: []seedSKU{{"100", 35}, {"110", 35}, {"120", 30}}},
	{Name: "中童针织毛衣裙", Category: "裙装", AgeGroup: model.AgeMiddle, Material: "腈纶混纺毛线", Season: model.SeasonAutumnWinter,
		Description: "粗棒针织纹理,搭配打底袜温暖过冬,下摆喇叭形活动自如。",
		Price:       12900, Sizes: []seedSKU{{"130", 25}, {"140", 25}, {"150", 25}}},
	{Name: "大童格纹百褶半身裙", Category: "裙装", AgeGroup: model.AgeBig, Material: "涤纶混纺", Season: model.SeasonSpringSummer,
		Description: "经典格纹,百褶裙摆,校园风百搭,配衬衫或卫衣都好看。",
		Price:       8900, Sizes: []seedSKU{{"150", 25}, {"160", 25}, {"165", 20}}},

	{Name: "婴儿外出斗篷外套", Category: "外套", AgeGroup: model.AgeBaby, Material: "摇粒绒", Season: model.SeasonAutumnWinter,
		Description: "斗篷式包裹,上下车不怕风,帽子上小鹿角造型萌趣可爱。",
		Price:       15900, Recommended: 1, Sizes: []seedSKU{{"73", 35}, {"80", 35}, {"90", 30}}},
	{Name: "小童轻量羽绒服(90绒)", Category: "外套", AgeGroup: model.AgeLittle, Material: "聚酯纤维+90%白鸭绒", Season: model.SeasonAutumnWinter,
		Description: "90% 白鸭绒充绒,轻若无物却很暖,防泼水面料不怕小雨。",
		Price:       39900, Recommended: 1, Sizes: []seedSKU{{"100", 25}, {"110", 25}, {"120", 20}}},
	{Name: "大童三合一冲锋衣", Category: "外套", AgeGroup: model.AgeBig, Material: "尼龙面料+PU 涂层", Season: model.SeasonAutumnWinter,
		Description: "三合一防风防水,户外郊游、上学通勤两相宜,反光 logo 更安全。",
		Price:       25900, Sizes: []seedSKU{{"150", 25}, {"160", 25}, {"165", 20}}},
	{Name: "中童水洗牛仔夹克", Category: "外套", AgeGroup: model.AgeMiddle, Material: "水洗牛仔布(棉)", Season: model.SeasonSpringSummer,
		Description: "经典水洗牛仔夹克,春秋过渡季百搭单品,刺绣小鹿点缀。",
		Price:       18900, Sizes: []seedSKU{{"130", 30}, {"140", 30}, {"150", 25}}},

	{Name: "儿童纯棉棒球帽", Category: "配饰", AgeGroup: model.AgeLittle, Material: "棉帆布", Season: model.SeasonSpringSummer,
		Description: "遮阳防晒,后帽扣可调头围,透气网眼不闷汗。",
		Price:       3500, Sizes: []seedSKU{{"小码(48cm)", 50}, {"中码(50cm)", 50}, {"大码(52cm)", 40}}},
	{Name: "大童针织围巾手套套装", Category: "配饰", AgeGroup: model.AgeBig, Material: "腈纶针织", Season: model.SeasonAutumnWinter,
		Description: "围巾+手套一套过冬,加绒内里,触屏指尖设计不影响玩手机。",
		Price:       5900, Sizes: []seedSKU{{"均码-雾蓝", 60}, {"均码-樱粉", 60}, {"均码-暖灰", 50}}},
}

// Seed 执行种子写入(各表为空时才写,幂等)。
func Seed(db *gorm.DB, users *repository.UserRepo) error {
	// 1. 分类(按名去重)
	var existingCats []model.Category
	if err := db.Find(&existingCats).Error; err != nil {
		return err
	}
	catID := make(map[string]uint64, len(categories))
	for _, c := range existingCats {
		catID[c.Name] = c.ID
	}
	for _, c := range categories {
		if _, ok := catID[c.Name]; ok {
			continue
		}
		nc := c
		if err := db.Create(&nc).Error; err != nil {
			return fmt.Errorf("写入分类 %s: %w", c.Name, err)
		}
		catID[nc.Name] = nc.ID
		logger.Info("种子:新增分类", "name", nc.Name, "id", nc.ID)
	}

	// 2. 商品 + SKU(商品表为空时全量写入)
	var productCount int64
	if err := db.Model(&model.Product{}).Count(&productCount).Error; err != nil {
		return err
	}
	if productCount == 0 {
		for i, sp := range products {
			pid, ok := catID[sp.Category]
			if !ok {
				continue
			}
			p := model.Product{
				CategoryID: pid, Name: sp.Name, AgeGroup: sp.AgeGroup,
				Material: sp.Material, Season: sp.Season, Description: sp.Description,
				Image: fmt.Sprintf("https://picsum.photos/seed/fawn-%d/600/600", i+1),
				Price: sp.Price, Status: model.ProductStatusOn, Recommended: sp.Recommended,
			}
			if err := db.Create(&p).Error; err != nil {
				return fmt.Errorf("写入商品 %s: %w", sp.Name, err)
			}
			for _, sk := range sp.Sizes {
				sku := model.SKU{ProductID: p.ID, Size: sk.Size, Stock: sk.Stock}
				if err := db.Create(&sku).Error; err != nil {
					return fmt.Errorf("写入 SKU %s/%s: %w", sp.Name, sk.Size, err)
				}
			}
			logger.Info("种子:新增商品", "name", sp.Name, "id", p.ID, "skus", len(sp.Sizes))
		}
	}

	// 3. 账号
	if n, err := users.CountByRole(model.RoleAdmin); err == nil && n == 0 {
		hash, herr := password.Hash(AdminPassword)
		if herr != nil {
			return herr
		}
		if err := users.Create(&model.User{
			Username: AdminUsername, PasswordHash: hash, Nickname: "店长小鹿", Role: model.RoleAdmin,
		}); err != nil {
			return err
		}
		logger.Info("种子:新增管理员账号", "username", AdminUsername)
	}
	if n, err := users.CountByRole(model.RoleUser); err == nil && n == 0 {
		hash, herr := password.Hash(DemoPassword)
		if herr != nil {
			return herr
		}
		if err := users.Create(&model.User{
			Username: DemoUsername, PasswordHash: hash, Nickname: "演示用户", Role: model.RoleUser,
		}); err != nil {
			return err
		}
		logger.Info("种子:新增演示用户账号", "username", DemoUsername)
	}
	return nil
}
