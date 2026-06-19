// ctp_tick/domain — 合约元数据纯逻辑（零外部依赖）
package domain

import (
	"strconv"
)

// ============================================================
// InstrumentMetadata — 单个期货合约的完整元数据
// 与 contracts/instrument.proto InstrumentMetadata 对应
// ============================================================

// InstrumentMetadata 合约元数据（domain 层中间结构）
type InstrumentMetadata struct {
	Symbol           string  `json:"symbol"`             // "MA2509",
	ProductID        string  `json:"product_id"`         // "MA",
	Exchange         string  `json:"exchange"`           // "CZCE",
	ProductNameCN    string  `json:"product_name_cn"`    // "甲醇",
	ContractYear     int     `json:"contract_year"`      // 2025,
	ContractMonth    int     `json:"contract_month"`     // 9,
	Multiplier       float64 `json:"multiplier"`         // 10,
	PriceTick        float64 `json:"price_tick"`         // 1.0,
	LongMarginRatio  float64 `json:"long_margin_ratio"`  // 0.08,
	ShortMarginRatio float64 `json:"short_margin_ratio"` // 0.08,
}

// ============================================================
// 产品 — 交易所映射表
// key=product_id（大小写敏感，按交易所实际注册代码）
// ============================================================

// productExchangeMap 品种所属交易所（非导出，防止外部并发写入导致 data race）
var productExchangeMap = map[string]string{
	// 郑商所 CZCE
	"MA": "CZCE", "TA": "CZCE", "FG": "CZCE", "SA": "CZCE",
	"UR": "CZCE", "PF": "CZCE", "CF": "CZCE", "SR": "CZCE",
	"OI": "CZCE", "RM": "CZCE", "ZC": "CZCE", "SM": "CZCE",
	"SF": "CZCE", "AP": "CZCE", "CJ": "CZCE", "CY": "CZCE",
	"JR": "CZCE", "LR": "CZCE", "ME": "CZCE", "RI": "CZCE",
	"WH": "CZCE", "PM": "CZCE",

	// 大商所 DCE
	"pp": "DCE", "l": "DCE", "v": "DCE",
	"eb": "DCE", "eg": "DCE", "pg": "DCE",
	"i": "DCE", "j": "DCE", "jm": "DCE",
	"m": "DCE", "y": "DCE", "a": "DCE",
	"b": "DCE", "c": "DCE", "cs": "DCE",
	"jd": "DCE", "lh": "DCE", "rr": "DCE",
	"fb": "DCE",

	// 上期所 SHFE
	"rb": "SHFE", "hc": "SHFE", "cu": "SHFE",
	"al": "SHFE", "zn": "SHFE", "ni": "SHFE",
	"sn": "SHFE", "pb": "SHFE", "ao": "SHFE",
	"ru": "SHFE", "fu": "SHFE", "bu": "SHFE",
	"sp": "SHFE", "ag": "SHFE", "au": "SHFE",
	"wr": "SHFE", "ss": "SHFE",

	// 能源中心 INE
	"sc": "INE", "lu": "INE", "bc": "INE", "nr": "INE",

	// 中金所 CFFEX
	"IF": "CFFEX", "IC": "CFFEX", "IH": "CFFEX", "IM": "CFFEX",
	"T": "CFFEX", "TF": "CFFEX", "TS": "CFFEX",

	// 广期所 GFEX
	"si": "GFEX", "lc": "GFEX",
}

// productNameCNMap 品种中文名映射（非导出）
var productNameCNMap = map[string]string{
	"MA": "甲醇", "TA": "PTA", "FG": "玻璃", "SA": "纯碱",
	"UR": "尿素", "PF": "短纤", "CF": "棉花", "SR": "白糖",
	"OI": "菜油", "RM": "菜粕", "ZC": "动力煤", "SM": "硅铁",
	"SF": "锰硅", "AP": "苹果", "CJ": "红枣", "CY": "棉纱",
	"JR": "粳稻", "LR": "晚籼稻", "RI": "早籼稻", "WH": "强麦",
	"PM": "普麦",

	"pp": "聚丙烯", "l": "聚乙烯", "v": "聚氯乙烯",
	"eb": "苯乙烯", "eg": "乙二醇", "pg": "液化气",
	"i": "铁矿石", "j": "焦炭", "jm": "焦煤",
	"m": "豆粕", "y": "豆油", "a": "豆一",
	"b": "豆二", "c": "玉米", "cs": "淀粉",
	"jd": "鸡蛋", "lh": "生猪", "rr": "粳米",

	"rb": "螺纹钢", "hc": "热轧卷板", "cu": "铜",
	"al": "铝", "zn": "锌", "ni": "镍",
	"sn": "锡", "pb": "铅", "ao": "氧化铝",
	"ru": "橡胶", "fu": "燃料油", "bu": "沥青",
	"sp": "纸浆", "ag": "白银", "au": "黄金",

	"sc": "原油", "lu": "低硫燃油", "bc": "国际铜", "nr": "20号胶",

	"IF": "沪深300", "IC": "中证500", "IH": "上证50", "IM": "中证1000",
	"T": "十年国债", "TF": "五年国债", "TS": "两年国债",

	"fb": "纤维板",
	"wr": "线材",
	"ss": "不锈钢",
	"si": "工业硅", "lc": "碳酸锂",
	"ME": "甲醇(旧)",
}

// ============================================================
// 合约规格（从 CTP 文档查询的常见值）
// ============================================================

// productMultiplier 品种合约乘数，每手吨数/克数（非导出）
var productMultiplier = map[string]float64{
	"MA": 10, "TA": 5, "FG": 20, "SA": 20,
	"UR": 20, "PF": 5, "CF": 5, "SR": 10,
	"OI": 10, "RM": 10,

	"pp": 5, "l": 5, "v": 5,
	"eb": 5, "eg": 10, "pg": 20,
	"i": 100, "j": 100, "jm": 60,
	"m": 10, "y": 10, "a": 10,

	"rb": 10, "hc": 10, "cu": 5,
	"al": 5, "zn": 5, "ni": 1,
	"sn": 1, "pb": 5, "ao": 20,
	"ru": 10, "fu": 10, "bu": 10,
	"sp": 10,

	"sc": 1000, "lu": 10, "bc": 5, "nr": 10,

	"IF": 300, "IC": 200, "IH": 300, "IM": 200,

	"si": 5, "lc": 1,
	"ZC": 100,
	"SM": 5,
	"SF": 5,
	"AP": 10,
	"CJ": 5,
	"CY": 5,
	"JR": 20,
	"LR": 20,
	"ME": 10,
	"RI": 20,
	"WH": 20,
	"PM": 50,
	"b":  10,
	"c":  10,
	"cs": 10,
	"jd": 10,
	"lh": 16,
	"rr": 10,
	"fb": 10,
	"ag": 15,
	"au": 1000,
	"wr": 10,
	"ss": 5,
	"T":  10000,
	"TF": 10000,
	"TS": 20000,
}

// productPriceTick 品种最小变动价位（非导出）
var productPriceTick = map[string]float64{
	"MA": 1, "TA": 2, "FG": 1, "SA": 1,
	"UR": 1, "PF": 2, "CF": 5, "SR": 1,
	"OI": 1, "RM": 1,

	"pp": 1, "l": 1, "v": 1,
	"eb": 1, "eg": 1, "pg": 1,
	"i": 0.5, "j": 0.5, "jm": 0.5,
	"m": 1, "y": 2, "a": 1,

	"rb": 1, "hc": 1, "cu": 10,
	"al": 5, "zn": 5, "ni": 10,
	"sn": 10, "pb": 5, "ao": 1,
	"ru": 5, "fu": 1, "bu": 2,
	"sp": 2,

	"sc": 0.1, "lu": 1, "bc": 10, "nr": 5,

	"IF": 0.2, "IC": 0.2, "IH": 0.2, "IM": 0.2,

	"si": 5, "lc": 50,
	"ZC": 0.2,
	"SM": 2,
	"SF": 2,
	"AP": 1,
	"CJ": 5,
	"CY": 5,
	"JR": 1,
	"LR": 1,
	"ME": 1,
	"RI": 1,
	"WH": 1,
	"PM": 1,
	"b":  1,
	"c":  1,
	"cs": 1,
	"jd": 1,
	"lh": 5,
	"rr": 1,
	"fb": 0.05,
	"ag": 1,
	"au": 0.02,
	"wr": 1,
	"ss": 5,
	"T":  0.005,
	"TF": 0.005,
	"TS": 0.005,
}

// productMarginRatio 品种默认保证金率（非导出）
var productMarginRatio = map[string]float64{
	"MA": 0.08, "TA": 0.07, "FG": 0.09, "SA": 0.09,
	"UR": 0.08, "PF": 0.08, "CF": 0.07, "SR": 0.07,
	"OI": 0.08, "RM": 0.08,

	"pp": 0.07, "l": 0.07, "v": 0.07,
	"eb": 0.08, "eg": 0.08, "pg": 0.08,
	"i": 0.11, "j": 0.11, "jm": 0.11,
	"m": 0.07, "y": 0.07, "a": 0.07,

	"rb": 0.08, "hc": 0.08, "cu": 0.08,
	"al": 0.08, "zn": 0.08, "ni": 0.10,
	"sn": 0.10, "pb": 0.08, "ao": 0.08,
	"ru": 0.08, "fu": 0.10, "bu": 0.10,
	"sp": 0.08,

	"si": 0.09, "lc": 0.12,
	"ZC": 0.12,
	"SM": 0.08,
	"SF": 0.08,
	"AP": 0.09,
	"CJ": 0.09,
	"CY": 0.07,
	"JR": 0.08,
	"LR": 0.08,
	"ME": 0.08,
	"RI": 0.08,
	"WH": 0.08,
	"PM": 0.08,
	"b":  0.07,
	"c":  0.07,
	"cs": 0.07,
	"jd": 0.08,
	"lh": 0.1,
	"rr": 0.07,
	"fb": 0.1,
	"ag": 0.1,
	"au": 0.08,
	"wr": 0.08,
	"ss": 0.08,
	"T":  0.02,
	"TF": 0.015,
	"TS": 0.01,
}

// ============================================================
// DeriveInstrumentInfo — 从合约代码推导元数据
// ============================================================

// DeriveInstrumentInfo 从 symbol 字符串推导 InstrumentMetadata
// 支持两种 CTP 命名惯例：
//   - 短式（CZCE 等）：产品代码 + 1 位年份 + 2 位月份，如 MA509
//   - 长式（SHFE/DCE 等）：产品代码 + 2 位年份 + 2 位月份，如 rb2510
func DeriveInstrumentInfo(symbol string) InstrumentMetadata {
	info := InstrumentMetadata{
		Symbol:           symbol,
		ProductID:        extractProductID(symbol),
		ContractYear:     0,
		ContractMonth:    0,
		Multiplier:       0,
		PriceTick:        0,
		LongMarginRatio:  0,
		ShortMarginRatio: 0,
	}

	// 合约年月
	year, month := parseContractDate(symbol, info.ProductID)
	info.ContractYear = year
	info.ContractMonth = month

	// 交易所
	if ex, ok := productExchangeMap[info.ProductID]; ok {
		info.Exchange = ex
	}

	// 中文名
	if name, ok := productNameCNMap[info.ProductID]; ok {
		info.ProductNameCN = name
	}

	// 合约规格
	if m, ok := productMultiplier[info.ProductID]; ok {
		info.Multiplier = m
	}
	if pt, ok := productPriceTick[info.ProductID]; ok {
		info.PriceTick = pt
	}
	if mr, ok := productMarginRatio[info.ProductID]; ok {
		info.LongMarginRatio = mr
		info.ShortMarginRatio = mr
	}

	return info
}

// DeriveAllInstrumentInfos 批量推导合约元数据
func DeriveAllInstrumentInfos(symbols []string) []InstrumentMetadata {
	result := make([]InstrumentMetadata, 0, len(symbols))
	seen := make(map[string]bool)
	for _, sym := range symbols {
		if seen[sym] {
			continue
		}
		seen[sym] = true
		result = append(result, DeriveInstrumentInfo(sym))
	}
	return result
}

// ============================================================
// 解析辅助函数
// ============================================================

// extractProductID 从合约代码中提取品种 ID
// 如 "MA2509" → "MA", "pp2509" → "pp", "rb2510" → "rb"
func extractProductID(symbol string) string {
	if symbol == "" {
		return ""
	}
	// 找到第一个数字的位置
	for i, c := range symbol {
		if c >= '0' && c <= '9' {
			return symbol[:i]
		}
	}
	return symbol
}

// parseContractDate 从合约代码解析年、月
// 返回 (year, month)
// 短式：product + 1位年份 + 2位月份 → 年份基准 2020
// 长式：product + 2位年份 + 2位月份
func parseContractDate(symbol, productID string) (int, int) {
	if symbol == "" || productID == "" {
		return 0, 0
	}

	// 去掉产品代码前缀，剩下数字部分
	numPart := symbol[len(productID):]
	if len(numPart) < 3 {
		return 0, 0
	}

	switch len(numPart) {
	case 3: // CZCE 短式: MA509 → 5 + 09
		yearDigit, err := strconv.Atoi(numPart[:1])
		if err != nil {
			return 0, 0
		}
		month, err := strconv.Atoi(numPart[1:])
		if err != nil || month < 1 || month > 12 {
			return 0, 0
		}
		return 2020 + yearDigit, month

	case 4: // 长式: rb2510 → 25 + 10
		yearTwo, err := strconv.Atoi(numPart[:2])
		if err != nil {
			return 0, 0
		}
		month, err := strconv.Atoi(numPart[2:])
		if err != nil || month < 1 || month > 12 {
			return 0, 0
		}
		return 2000 + yearTwo, month

	case 5: // 商品+期权组合等罕见格式
		yearTwo, err := strconv.Atoi(numPart[:2])
		if err != nil {
			return 0, 0
		}
		month, err := strconv.Atoi(numPart[2:4])
		if err != nil || month < 1 || month > 12 {
			return 0, 0
		}
		return 2000 + yearTwo, month

	default:
		return 0, 0
	}
}
