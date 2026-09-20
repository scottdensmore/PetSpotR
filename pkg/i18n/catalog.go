package i18n

import (
	"fmt"
	"strconv"
	"strings"
)

// Supported locale constants.
const (
	LocaleEN   = "en"
	LocaleES   = "es"
	LocaleVI   = "vi"
	LocaleZHCN = "zh-CN"
	LocaleTL   = "tl"

	DefaultLocale = LocaleEN
	CookieName    = "petspotr_locale"
)

// SupportedLocales lists the locales PetSpotR officially provides translations for.
var SupportedLocales = []string{
	LocaleEN,
	LocaleES,
	LocaleVI,
	LocaleZHCN,
	LocaleTL,
}

// LocaleDisplayName maps a locale code to its native display name.
var LocaleDisplayName = map[string]string{
	LocaleEN:   "English",
	LocaleES:   "Español",
	LocaleVI:   "Tiếng Việt",
	LocaleZHCN: "简体中文",
	LocaleTL:   "Tagalog",
}

// IsSupported returns true if the locale code is supported.
func IsSupported(locale string) bool {
	switch locale {
	case LocaleEN, LocaleES, LocaleVI, LocaleZHCN, LocaleTL:
		return true
	default:
		return false
	}
}

// NormalizeLocale standardizes language codes (case-insensitivity, regional tags).
func NormalizeLocale(locale string) string {
	cleaned := strings.TrimSpace(locale)
	if cleaned == "" {
		return DefaultLocale
	}

	lower := strings.ToLower(cleaned)
	switch {
	case lower == "en" || strings.HasPrefix(lower, "en-") || strings.HasPrefix(lower, "en_"):
		return LocaleEN
	case lower == "es" || strings.HasPrefix(lower, "es-") || strings.HasPrefix(lower, "es_"):
		return LocaleES
	case lower == "vi" || strings.HasPrefix(lower, "vi-") || strings.HasPrefix(lower, "vi_"):
		return LocaleVI
	case lower == "zh-cn" || lower == "zh_cn" || lower == "zh" || strings.HasPrefix(lower, "zh-"):
		return LocaleZHCN
	case lower == "tl" || strings.HasPrefix(lower, "tl-") || lower == "fil" || strings.HasPrefix(lower, "fil-"):
		return LocaleTL
	default:
		return DefaultLocale
	}
}

// ResolveLocale resolves the preferred locale given query param, cookie, and Accept-Language header.
func ResolveLocale(query, cookie, acceptHeader string) string {
	if q := strings.TrimSpace(query); q != "" {
		norm := NormalizeLocale(q)
		if IsSupported(norm) && (norm == q || strings.EqualFold(norm, q) || strings.HasPrefix(strings.ToLower(q), norm)) {
			return norm
		}
	}

	if c := strings.TrimSpace(cookie); c != "" {
		norm := NormalizeLocale(c)
		if IsSupported(norm) && (norm == c || strings.EqualFold(norm, c) || strings.HasPrefix(strings.ToLower(c), norm)) {
			return norm
		}
	}

	if accept := strings.TrimSpace(acceptHeader); accept != "" {
		if resolved := parseAcceptLanguage(accept); resolved != "" {
			return resolved
		}
	}

	return DefaultLocale
}

type langPreference struct {
	locale  string
	quality float64
}

func parseAcceptLanguage(header string) string {
	parts := strings.Split(header, ",")
	var prefs []langPreference

	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}

		subparts := strings.Split(item, ";")
		lang := strings.TrimSpace(subparts[0])
		q := 1.0

		for _, param := range subparts[1:] {
			param = strings.TrimSpace(param)
			if strings.HasPrefix(param, "q=") {
				if val, err := strconv.ParseFloat(param[2:], 64); err == nil {
					q = val
				}
			}
		}

		norm := NormalizeLocale(lang)
		if IsSupported(norm) {
			prefs = append(prefs, langPreference{locale: norm, quality: q})
		}
	}

	var bestLocale string
	bestQ := -1.0
	for _, p := range prefs {
		if p.quality > bestQ {
			bestQ = p.quality
			bestLocale = p.locale
		}
	}

	return bestLocale
}

// Translate retrieves the translated string for a given key and locale, falling back to English.
func Translate(locale, key string, args ...any) string {
	norm := NormalizeLocale(locale)
	dict, exists := translations[norm]
	var text string
	if exists {
		text = dict[key]
	}

	if text == "" {
		// Fallback to default English
		if defaultDict, ok := translations[DefaultLocale]; ok {
			text = defaultDict[key]
		}
	}

	if text == "" {
		text = key
	}

	if len(args) > 0 {
		return fmt.Sprintf(text, args...)
	}
	return text
}

// TranslatePlural returns a pluralized translation based on count.
func TranslatePlural(locale, key string, count int, args ...any) string {
	suffix := ".other"
	if count == 1 {
		suffix = ".one"
	}

	pluralKey := key + suffix
	norm := NormalizeLocale(locale)
	dict, exists := translations[norm]
	var text string
	if exists {
		text = dict[pluralKey]
	}
	if text == "" {
		if defaultDict, ok := translations[DefaultLocale]; ok {
			text = defaultDict[pluralKey]
		}
	}
	if text == "" {
		// Try root key
		text = Translate(locale, key)
	}

	if len(args) > 0 {
		return fmt.Sprintf(text, args...)
	}
	if strings.Contains(text, "%") {
		return fmt.Sprintf(text, count)
	}
	return text
}

// Translation dictionaries.
var translations = map[string]map[string]string{
	LocaleEN: {
		"test.english_only":                 "Only in English",
		"nav.home":                          "Home",
		"nav.directory":                     "Pet Directory",
		"nav.matches":                       "Match Dashboard",
		"nav.analytics":                     "Shelter Analytics",
		"nav.report_lost":                   "Report Lost Pet",
		"nav.report_found":                  "Found a Pet",
		"nav.language":                      "Language",
		"hero.title":                        "Reuniting Lost Pets with AI Vision Intelligence",
		"hero.subtitle":                     "PetSpotR combines event-driven microservices with Gemma AI vision models and geospatial indexing to automatically match reported lost pets with found animals.",
		"hero.report_missing":               "Report Missing Pet",
		"hero.found_pet":                    "I Found a Pet",
		"directory.title":                   "Public Pet Directory",
		"directory.subtitle":                "Community-wide active lost and found pet reports with AI-indexed attributes.",
		"directory.species":                 "Species",
		"directory.all_species":             "All Species",
		"directory.dogs":                    "Dogs",
		"directory.cats":                    "Cats",
		"directory.other":                   "Other Animals",
		"directory.status":                  "Report Status",
		"directory.all_reports":             "All Active Reports",
		"directory.lost_pets":               "Lost Pets",
		"directory.found_pets":              "Found Pets",
		"directory.search":                  "Search Query",
		"directory.search_placeholder":      "Search breed, name, location...",
		"directory.near_me":                 "Near Me",
		"directory.location_set":            "Location Set",
		"directory.radius":                  "Search radius",
		"directory.lost_badge":              "Lost Pet",
		"directory.found_badge":             "Found Pet",
		"sighting.modal_title":              "👁️ Report Pet Sighting",
		"sighting.close":                    "Close sighting modal",
		"sighting.intro":                    "Report where and when you spotted this pet to help map their live movement trajectory and search perimeter.",
		"sighting.photo_label":              "Photo (Optional — auto-detects EXIF location & time)",
		"sighting.photo_dropzone":           "Drag & drop photo or click to upload",
		"sighting.photo_help":               "Extracts GPS & capture time from photo EXIF",
		"sighting.use_location":             "Use My Location",
		"sighting.gps_not_acquired":         "GPS not acquired",
		"sighting.location_label":           "Location / Landmarks",
		"sighting.location_placeholder":     "e.g. Near Pike Place Market, 4th and Olive Way",
		"sighting.when_seen":                "When Seen?",
		"sighting.just_now":                 "Just now",
		"sighting.heading_direction":        "Heading Direction",
		"sighting.notes_label":              "Witness Notes / Demeanor",
		"sighting.notes_placeholder":        "e.g. Trotting quickly, wearing red harness, seemed spooked by sirens",
		"sighting.voice_memo_title":         "Voice Sighting Note",
		"sighting.voice_memo_record":        "Record Voice Memo",
		"sighting.voice_memo_recording":     "Recording... (Max 15s)",
		"sighting.voice_memo_stop":          "Stop Recording",
		"sighting.voice_memo_preview":       "Audio Note Preview",
		"sighting.voice_memo_delete":        "Discard Voice Memo",
		"sighting.voice_memo_duration":      "Max 15 seconds",
		"sighting.cancel":                   "Cancel",
		"sighting.submit":                   "Submit Sighting",
		"sighting.count.one":                "%d sighting",
		"sighting.count.other":              "%d sightings",
		"lost_pets.count.one":               "%d lost pet",
		"lost_pets.count.other":             "%d lost pets",
		"matches.count.one":                 "%d match",
		"matches.count.other":               "%d matches",
		"aria.language_changed":             "Language changed to %s",
		"aria.voice_memo_recording_started": "Voice memo recording started. Maximum 15 seconds.",
		"aria.voice_memo_recording_stopped": "Voice memo recording stopped.",
		"aria.voice_memo_uploaded":          "Voice memo uploaded successfully.",
		"aria.sighting_submitted":           "Pet sighting reported successfully.",
	},
	LocaleES: {
		"nav.home":                          "Inicio",
		"nav.directory":                     "Directorio de Mascotas",
		"nav.matches":                       "Panel de Coincidencias",
		"nav.analytics":                     "Estadísticas de Refugio",
		"nav.report_lost":                   "Reportar Mascota Perdida",
		"nav.report_found":                  "Encontré una Mascota",
		"nav.language":                      "Idioma",
		"hero.title":                        "Reuniendo Mascotas Perdidas con Inteligencia Visual IA",
		"hero.subtitle":                     "PetSpotR combina microservicios impulsados por eventos con modelos de visión Gemma AI e indexación geoespacial para conectar automáticamente mascotas perdidas con animales encontrados.",
		"hero.report_missing":               "Reportar Mascota Desaparecida",
		"hero.found_pet":                    "Encontré una Mascota",
		"directory.title":                   "Directorio Público de Mascotas",
		"directory.subtitle":                "Informes activos de mascotas perdidas y encontradas en toda la comunidad con atributos indexados por IA.",
		"directory.species":                 "Especie",
		"directory.all_species":             "Todas las especies",
		"directory.dogs":                    "Perros",
		"directory.cats":                    "Gatos",
		"directory.other":                   "Otros Animales",
		"directory.status":                  "Estado del Reporte",
		"directory.all_reports":             "Todos los Informes Activos",
		"directory.lost_pets":               "Mascotas Perdidas",
		"directory.found_pets":              "Mascotas Encontradas",
		"directory.search":                  "Consulta de Búsqueda",
		"directory.search_placeholder":      "Buscar raza, nombre, ubicación...",
		"directory.near_me":                 "Cerca de Mí",
		"directory.location_set":            "Ubicación Fijada",
		"directory.radius":                  "Radio de búsqueda",
		"directory.lost_badge":              "Mascota Perdida",
		"directory.found_badge":             "Mascota Encontrada",
		"sighting.modal_title":              "👁️ Reportar Avistamiento de Mascota",
		"sighting.close":                    "Cerrar modal de avistamiento",
		"sighting.intro":                    "Reporte dónde y cuándo vio a esta mascota para ayudar a trazar su trayectoria de movimiento en vivo y el perímetro de búsqueda.",
		"sighting.photo_label":              "Foto (Opcional: detecta automáticamente ubicación y hora EXIF)",
		"sighting.photo_dropzone":           "Arrastre y suelte una foto o haga clic para subir",
		"sighting.photo_help":               "Extrae GPS y hora de captura desde el EXIF de la foto",
		"sighting.use_location":             "Usar Mi Ubicación",
		"sighting.gps_not_acquired":         "GPS no adquirido",
		"sighting.location_label":           "Ubicación / Puntos de Referencia",
		"sighting.location_placeholder":     "ej. Cerca de Plaza Central, Calle 4 y Avenida Olivo",
		"sighting.when_seen":                "¿Cuándo se vio?",
		"sighting.just_now":                 "Justo ahora",
		"sighting.heading_direction":        "Dirección del Desplazamiento",
		"sighting.notes_label":              "Notas del Testigo / Comportamiento",
		"sighting.notes_placeholder":        "ej. Trotando rápidamente, con arnés rojo, parecía asustado",
		"sighting.voice_memo_title":         "Nota de Voz del Avistamiento",
		"sighting.voice_memo_record":        "Grabar Nota de Voz",
		"sighting.voice_memo_recording":     "Grabando... (Máx 15s)",
		"sighting.voice_memo_stop":          "Detener Grabación",
		"sighting.voice_memo_preview":       "Vista Previa de la Nota de Audio",
		"sighting.voice_memo_delete":        "Descartar Nota de Voz",
		"sighting.voice_memo_duration":      "Máximo 15 segundos",
		"sighting.cancel":                   "Cancelar",
		"sighting.submit":                   "Enviar Avistamiento",
		"sighting.count.one":                "%d avistamiento",
		"sighting.count.other":              "%d avistamientos",
		"lost_pets.count.one":               "%d mascota perdida",
		"lost_pets.count.other":             "%d mascotas perdidas",
		"matches.count.one":                 "%d coincidencia",
		"matches.count.other":               "%d coincidencias",
		"aria.language_changed":             "Idioma cambiado a %s",
		"aria.voice_memo_recording_started": "Grabación de nota de voz iniciada. Máximo 15 segundos.",
		"aria.voice_memo_recording_stopped": "Grabación de nota de voz detenida.",
		"aria.voice_memo_uploaded":          "Nota de voz subida con éxito.",
		"aria.sighting_submitted":           "Avistamiento de mascota reportado con éxito.",
	},
	LocaleVI: {
		"nav.home":                          "Trang chủ",
		"nav.directory":                     "Danh mục thú cưng",
		"nav.matches":                       "Bảng đối sánh",
		"nav.analytics":                     "Phân tích trạm cứu hộ",
		"nav.report_lost":                   "Báo thú cưng thất lạc",
		"nav.report_found":                  "Tìm thấy thú cưng",
		"nav.language":                      "Ngôn ngữ",
		"hero.title":                        "Đoàn tụ thú cưng thất lạc với Trí tuệ Thị giác AI",
		"hero.subtitle":                     "PetSpotR kết hợp microservices hướng sự kiện với mô hình thị giác Gemma AI và lập chỉ mục không gian địa lý để tự động đối sánh thú cưng thất lạc với động vật được tìm thấy.",
		"hero.report_missing":               "Báo thú cưng mất tích",
		"hero.found_pet":                    "Tôi tìm thấy thú cưng",
		"directory.title":                   "Danh mục Thú cưng Công khai",
		"directory.subtitle":                "Các báo cáo thú cưng thất lạc và tìm thấy đang hoạt động trên toàn cộng đồng.",
		"directory.species":                 "Loài",
		"directory.all_species":             "Tất cả loài",
		"directory.dogs":                    "Chó",
		"directory.cats":                    "Mèo",
		"directory.other":                   "Động vật khác",
		"directory.status":                  "Trạng thái Báo cáo",
		"directory.all_reports":             "Tất cả Báo cáo Hoạt động",
		"directory.lost_pets":               "Thú cưng Thất lạc",
		"directory.found_pets":              "Thú cưng Tìm thấy",
		"directory.search":                  "Truy vấn Tìm kiếm",
		"directory.search_placeholder":      "Tìm kiếm giống, tên, địa điểm...",
		"directory.near_me":                 "Gần tôi",
		"directory.location_set":            "Đã đặt vị trí",
		"directory.radius":                  "Bán kính tìm kiếm",
		"directory.lost_badge":              "Thú cưng Thất lạc",
		"directory.found_badge":             "Thú cưng Tìm thấy",
		"sighting.modal_title":              "👁️ Báo cáo nhìn thấy thú cưng",
		"sighting.close":                    "Đóng hộp thoại nhìn thấy",
		"sighting.intro":                    "Báo cáo nơi và thời điểm bạn phát hiện thú cưng này để hỗ trợ lập bản đồ di chuyển.",
		"sighting.photo_label":              "Ảnh (Tùy chọn — tự động phát hiện vị trí & thời gian EXIF)",
		"sighting.photo_dropzone":           "Kéo & thả ảnh hoặc nhấp để tải lên",
		"sighting.photo_help":               "Trích xuất GPS & thời gian chụp từ EXIF ảnh",
		"sighting.use_location":             "Sử dụng Vị trí của tôi",
		"sighting.gps_not_acquired":         "Chưa nhận được GPS",
		"sighting.location_label":           "Địa điểm / Mốc chuẩn",
		"sighting.location_placeholder":     "vd: Gần chợ Bến Thành, Đường Lê Lợi",
		"sighting.when_seen":                "Nhìn thấy khi nào?",
		"sighting.just_now":                 "Vừa mới đây",
		"sighting.heading_direction":        "Hướng di chuyển",
		"sighting.notes_label":              "Ghi chú của người chứng kiến",
		"sighting.notes_placeholder":        "vd: Chạy nhanh, đeo vòng đỏ, có vẻ sợ tiếng còi",
		"sighting.voice_memo_title":         "Ghi chú giọng nói khi nhìn thấy",
		"sighting.voice_memo_record":        "Ghi âm ghi chú",
		"sighting.voice_memo_recording":     "Đang ghi âm... (Tối đa 15 giây)",
		"sighting.voice_memo_stop":          "Dừng ghi âm",
		"sighting.voice_memo_preview":       "Nghe lại ghi chú âm thanh",
		"sighting.voice_memo_delete":        "Hủy ghi chú",
		"sighting.voice_memo_duration":      "Tối đa 15 giây",
		"sighting.cancel":                   "Hủy",
		"sighting.submit":                   "Gửi báo cáo",
		"sighting.count.one":                "%d lần nhìn thấy",
		"sighting.count.other":              "%d lần nhìn thấy",
		"lost_pets.count.one":               "%d thú cưng thất lạc",
		"lost_pets.count.other":             "%d thú cưng thất lạc",
		"matches.count.one":                 "%d đối sánh",
		"matches.count.other":               "%d đối sánh",
		"aria.language_changed":             "Đã chuyển ngôn ngữ sang %s",
		"aria.voice_memo_recording_started": "Đã bắt đầu ghi âm. Tối đa 15 giây.",
		"aria.voice_memo_recording_stopped": "Đã dừng ghi âm.",
		"aria.voice_memo_uploaded":          "Đã tải lên bản ghi âm thành công.",
		"aria.sighting_submitted":           "Đã gửi báo cáo nhìn thấy thú cưng thành công.",
	},
	LocaleZHCN: {
		"nav.home":                          "首页",
		"nav.directory":                     "宠物名录",
		"nav.matches":                       "匹配面板",
		"nav.analytics":                     "收容所分析",
		"nav.report_lost":                   "报告丢失宠物",
		"nav.report_found":                  "发现宠物",
		"nav.language":                      "语言",
		"hero.title":                        "依托 AI 视觉智能让走失宠物早日团聚",
		"hero.subtitle":                     "PetSpotR 结合事件驱动微服务与 Gemma AI 视觉模型及地理空间索引，自动将走失宠物与发现的动物进行精准匹配。",
		"hero.report_missing":               "报告走失宠物",
		"hero.found_pet":                    "我发现了宠物",
		"directory.title":                   "公共宠物名录",
		"directory.subtitle":                "全社区活跃的走失与发现宠物报告，具备 AI 索引属性。",
		"directory.species":                 "物种",
		"directory.all_species":             "所有物种",
		"directory.dogs":                    "狗",
		"directory.cats":                    "猫",
		"directory.other":                   "其他动物",
		"directory.status":                  "报告状态",
		"directory.all_reports":             "所有有效报告",
		"directory.lost_pets":               "丢失宠物",
		"directory.found_pets":              "拾获宠物",
		"directory.search":                  "搜索查询",
		"directory.search_placeholder":      "搜索品种、名字、地点...",
		"directory.near_me":                 "我附近",
		"directory.location_set":            "位置已设置",
		"directory.radius":                  "搜索半径",
		"directory.lost_badge":              "走失宠物",
		"directory.found_badge":             "拾获宠物",
		"sighting.modal_title":              "👁️ 报告宠物目击",
		"sighting.close":                    "关闭目击弹窗",
		"sighting.intro":                    "报告您看到该宠物的地点和时间，帮助绘制实时行动轨迹与搜索范围。",
		"sighting.photo_label":              "照片（可选 — 自动检测 EXIF 位置与拍摄时间）",
		"sighting.photo_dropzone":           "拖放照片或点击上传",
		"sighting.photo_help":               "从照片 EXIF 中提取 GPS 与拍摄时间",
		"sighting.use_location":             "使用我的位置",
		"sighting.gps_not_acquired":         "未获取 GPS",
		"sighting.location_label":           "位置 / 地标",
		"sighting.location_placeholder":     "例如：人民广场附近，南京东路与西藏中路路口",
		"sighting.when_seen":                "目击时间",
		"sighting.just_now":                 "刚才",
		"sighting.heading_direction":        "行进方向",
		"sighting.notes_label":              "目击者备注 / 状态",
		"sighting.notes_placeholder":        "例如：快速小跑，佩戴红色胸背带，似乎被汽笛声惊吓",
		"sighting.voice_memo_title":         "目击语音备注",
		"sighting.voice_memo_record":        "录制语音备注",
		"sighting.voice_memo_recording":     "录音中...（最长 15 秒）",
		"sighting.voice_memo_stop":          "停止录制",
		"sighting.voice_memo_preview":       "音频备注预览",
		"sighting.voice_memo_delete":        "放弃语音备注",
		"sighting.voice_memo_duration":      "最长 15 秒",
		"sighting.cancel":                   "取消",
		"sighting.submit":                   "提交目击报告",
		"sighting.count.one":                "%d 次目击",
		"sighting.count.other":              "%d 次目击",
		"lost_pets.count.one":               "%d 只走失宠物",
		"lost_pets.count.other":             "%d 只走失宠物",
		"matches.count.one":                 "%d 条匹配",
		"matches.count.other":               "%d 条匹配",
		"aria.language_changed":             "语言已更改为 %s",
		"aria.voice_memo_recording_started": "语音备注录制已开始。最长 15 秒。",
		"aria.voice_memo_recording_stopped": "语音备注录制已停止。",
		"aria.voice_memo_uploaded":          "语音备注上传成功。",
		"aria.sighting_submitted":           "宠物目击报告提交成功。",
	},
	LocaleTL: {
		"nav.home":                          "Tahanan",
		"nav.directory":                     "Direktoryo ng Alagang Hayop",
		"nav.matches":                       "Dashboard ng Pagtutugma",
		"nav.analytics":                     "Analitika ng Kanlungan",
		"nav.report_lost":                   "I-ulat ang Nawawalang Alaga",
		"nav.report_found":                  "Nakakita ng Alagang Hayop",
		"nav.language":                      "Wika",
		"hero.title":                        "Muling Paghaharap ng mga Nawawalang Alaga Gamit ang AI Vision",
		"hero.subtitle":                     "Pinagsasama ng PetSpotR ang event-driven microservices at Gemma AI vision models para awtomatikong itugma ang nawawala at nakitang alaga.",
		"hero.report_missing":               "I-ulat ang Nawawalang Alaga",
		"hero.found_pet":                    "Nakakita Ako ng Alaga",
		"directory.title":                   "Pampublikong Direktoryo ng Alagang Hayop",
		"directory.subtitle":                "Aktibong mga ulat ng nawawala at nakitang alaga sa buong komunidad.",
		"directory.species":                 "Uri ng Hayop",
		"directory.all_species":             "Lahat ng Uri",
		"directory.dogs":                    "Mga Aso",
		"directory.cats":                    "Mga Pusa",
		"directory.other":                   "Iba Pang Hayop",
		"directory.status":                  "Katayuan ng Ulat",
		"directory.all_reports":             "Lahat ng Aktibong Ulat",
		"directory.lost_pets":               "Nawawalang Alaga",
		"directory.found_pets":              "Nakitang Alaga",
		"directory.search":                  "Paghahanap",
		"directory.search_placeholder":      "Maghanap ng lahi, pangalan, lokasyon...",
		"directory.near_me":                 "Malapit sa Akin",
		"directory.location_set":            "Naitakda ang Lokasyon",
		"directory.radius":                  "Lapad ng Paghahanap",
		"directory.lost_badge":              "Nawawalang Alaga",
		"directory.found_badge":             "Nakitang Alaga",
		"sighting.modal_title":              "👁️ I-ulat ang Pagkakita sa Alaga",
		"sighting.close":                    "Isara ang modal ng sighting",
		"sighting.intro":                    "I-ulat kung saan at kailan mo nakita ang alagang ito upang makatulong sa pagmamapa ng trajectory.",
		"sighting.photo_label":              "Larawan (Opsyonal — awtomatikong sinusuri ang lokasyon at oras ng EXIF)",
		"sighting.photo_dropzone":           "I-drag at i-drop ang larawan o mag-click upang mag-upload",
		"sighting.photo_help":               "Kinukuha ang GPS at oras mula sa EXIF",
		"sighting.use_location":             "Gamitin ang Aking Lokasyon",
		"sighting.gps_not_acquired":         "Hindi nakuha ang GPS",
		"sighting.location_label":           "Lokasyon / Landmark",
		"sighting.location_placeholder":     "hal. Malapit sa Palengke, Kanto ng Rizal St.",
		"sighting.when_seen":                "Kailan Nakita?",
		"sighting.just_now":                 "Ngayon lang",
		"sighting.heading_direction":        "Direksyon ng Pagpunta",
		"sighting.notes_label":              "Mga Tala ng Saksi",
		"sighting.notes_placeholder":        "hal. Mabilis na tumatakbo, may pulang harness, natatakot sa busina",
		"sighting.voice_memo_title":         "Voice Note ng Pagkakita",
		"sighting.voice_memo_record":        "Mag-record ng Voice Memo",
		"sighting.voice_memo_recording":     "Nagre-record... (Max 15 segundo)",
		"sighting.voice_memo_stop":          "Itigil ang Pag-record",
		"sighting.voice_memo_preview":       "Pakinggan ang Audio Note",
		"sighting.voice_memo_delete":        "Tanggalin ang Voice Memo",
		"sighting.voice_memo_duration":      "Hanggang 15 segundo",
		"sighting.cancel":                   "Kanselahin",
		"sighting.submit":                   "Isumite ang Sighting",
		"sighting.count.one":                "%d sighting",
		"sighting.count.other":              "%d mga sighting",
		"lost_pets.count.one":               "%d nawawalang alaga",
		"lost_pets.count.other":             "%d nawawalang mga alaga",
		"matches.count.one":                 "%d tugma",
		"matches.count.other":               "%d mga tugma",
		"aria.language_changed":             "Pinalitan ang wika sa %s",
		"aria.voice_memo_recording_started": "Nagsimula ang pag-record ng voice memo. Hanggang 15 segundo.",
		"aria.voice_memo_recording_stopped": "Itinigil ang pag-record ng voice memo.",
		"aria.voice_memo_uploaded":          "Matagumpay na na-upload ang voice memo.",
		"aria.sighting_submitted":           "Matagumpay na naitala ang sighting ng alaga.",
	},
}
