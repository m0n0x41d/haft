package cli

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
)

// Харнесс качества FPF-retrieval.
//
// Зачем он нужен именно в такой форме. В индексе лежат 2428 пар
// «фраза -> единица», написанных авторами источника, но мерить поиск на них
// напрямую циркулярно: authored_phrase и heading_keyword — это тиры самого
// ретривала, и recall@1 вышел бы почти идеальным по построению.
//
// Наблюдаемый режим отказа вокабулярный: инженер спрашивает обычными словами,
// источник написан точной лексикой FPF. Поэтому измеряется устойчивость к
// расхождению словаря, а не точное совпадение:
//
//   - набор paraphrase — рукописные вопросы намеренно ЧУЖИМИ словами;
//   - набор degraded  — авторские фразы, механически лишённые слов заголовка
//     целевой единицы, то есть тех самых, которые делают попадание тривиальным.
//
// Харнесс печатает recall@1/@3/@10 и падает только ниже консервативного пола.
// Пол — защита от регрессии, а не цель: расти он должен вместе с retrieval.

type fpfRecallCase struct {
	text string
	want string // unit_id ожидаемой единицы
	// knownContext несёт точные английские термины рядом с сохранённым
	// оригиналом запроса. Для неанглийских концернов это обязательная часть
	// контракта, а не украшение: встроенный источник английский, и сырой
	// русский запрос возвращает ноль кандидатов. Кейсы ниже поэтому меряют
	// работающий путь, а не заведомо пустой.
	knownContext []string
}

// Рукописные перефразировки: обычный инженерный язык, лексика FPF не
// используется намеренно. Цель — 16 practical_use_card, потому что именно они
// заявлены как точка входа для ситуации.
var fpfParaphraseCases = []fpfRecallCase{
	{"we are about to ship something we cannot roll back, what should we check first", "readme:practical_use_card:costly-action", nil},
	{"before doing something irreversible and expensive what is missing", "readme:practical_use_card:costly-action", nil},
	{"мы собираемся сделать необратимое и дорогое действие, чего не хватает", "readme:practical_use_card:costly-action",
		[]string{"costly irreversible action commitment evidence"}},

	{"we have several possible approaches and need to pick one fairly", "readme:practical_use_card:option-comparison", nil},
	{"how do I choose between candidate solutions without hiding the tradeoff", "readme:practical_use_card:option-comparison", nil},
	{"как честно выбрать между несколькими вариантами решения", "readme:practical_use_card:option-comparison",
		[]string{"compare alternatives selection parity"}},

	{"the team keeps optimizing but nobody agreed what better means", "readme:practical_use_card:improvement", nil},
	{"we want to make it better but have no way to tell if we did", "readme:practical_use_card:improvement", nil},

	{"someone reports a vague pain and we do not know the real question yet", "readme:practical_use_card:problem-shaping", nil},
	{"there is pressure to act but the problem is not stated", "readme:practical_use_card:problem-shaping", nil},

	{"is this thing actually a system or just a pile of parts", "readme:practical_use_card:system-recognition", nil},
	{"the word we use might mean a system or might mean a document", "readme:practical_use_card:system-recognition", nil},

	{"which components belong inside the boundary and which only touch it", "readme:practical_use_card:system-delimitation", nil},
	{"where does our service end and the environment begin", "readme:practical_use_card:system-delimitation", nil},

	{"someone claims one change caused another and wants to act on it", "readme:practical_use_card:causal-use", nil},
	{"we are about to treat correlation as if it were a lever", "readme:practical_use_card:causal-use", nil},

	{"the sentence reads well but I cannot tell what it asserts", "readme:practical_use_card:wording", nil},
	{"fluent text that leaves the reader unsure what was actually said", "readme:practical_use_card:wording", nil},

	{"we need to write something another team can actually act on", "readme:practical_use_card:working-documents", nil},
	{"producing a procedure someone else will follow", "readme:practical_use_card:working-documents", nil},

	{"how stale is this number and when does it stop being true", "readme:practical_use_card:time", nil},
	{"claims about how fresh the data is and how long it stays valid", "readme:practical_use_card:time", nil},

	{"we need a stable name for this value that readers will understand", "readme:practical_use_card:naming", nil},

	{"the dashboard shows a thing but I lost track of what it describes", "readme:practical_use_card:description-use", nil},

	{"what does the field currently offer before we build our own", "readme:practical_use_card:sota-portfolio", nil},

	{"designing the shape of the system from the pressure on it", "readme:practical_use_card:architecture", nil},
}

func TestFPFQueryRecallEvalParaphrase(t *testing.T) {
	if testing.Short() {
		t.Skip("FPF recall eval skipped in -short")
	}
	db, cleanup, err := openFPFDBFunc()
	if err != nil {
		t.Skipf("embedded FPF index unavailable: %v", err)
	}
	defer cleanup()

	report := evaluateFPFRecall(t, db, fpfParaphraseCases)
	report.log(t, "paraphrase")

	// Пол защищает от регрессии. Он ниже текущего измерения намеренно:
	// цель харнесса — заметить ухудшение, а не зафиксировать достижение.
	if report.at10 < 0.30 {
		t.Errorf("paraphrase R@10 = %.2f упал ниже пола 0.30", report.at10)
	}
}

func TestFPFQueryRecallEvalDegradedAuthoredPhrases(t *testing.T) {
	if testing.Short() {
		t.Skip("FPF recall eval skipped in -short")
	}
	db, cleanup, err := openFPFDBFunc()
	if err != nil {
		t.Skipf("embedded FPF index unavailable: %v", err)
	}
	defer cleanup()

	cases := buildDegradedAuthoredPhraseCases(t, db, 120)
	if len(cases) == 0 {
		t.Skip("no degraded authored-phrase cases could be built")
	}
	report := evaluateFPFRecall(t, db, cases)
	report.log(t, "degraded-authored-phrase")

	if report.at10 < 0.20 {
		t.Errorf("degraded R@10 = %.2f упал ниже пола 0.20", report.at10)
	}
}

type fpfRecallReport struct {
	total          int
	at1, at3, at10 float64
	misses         []string
}

func (report fpfRecallReport) log(t *testing.T, label string) {
	t.Helper()
	t.Logf(
		"%s: n=%d  R@1=%.2f  R@3=%.2f  R@10=%.2f",
		label, report.total, report.at1, report.at3, report.at10,
	)
	for index, miss := range report.misses {
		if index >= 8 {
			t.Logf("  ... и ещё %d промахов", len(report.misses)-8)
			break
		}
		t.Logf("  промах: %s", miss)
	}
}

func evaluateFPFRecall(
	t *testing.T,
	db *sql.DB,
	cases []fpfRecallCase,
) fpfRecallReport {
	t.Helper()
	index := fpf.NewSQLiteQueryIndex(db)
	report := fpfRecallReport{total: len(cases)}
	var hits1, hits3, hits10 int

	for _, testCase := range cases {
		result, err := fpf.Query(index, fpf.ConcernQuery{Text: testCase.text, KnownContext: testCase.knownContext})
		if err != nil {
			t.Fatalf("query %q: %v", testCase.text, err)
		}
		ranked := rankedUnitIDs(result)
		position := -1
		for rank, unitID := range ranked {
			if unitID == testCase.want {
				position = rank
				break
			}
		}
		switch {
		case position < 0:
			report.misses = append(
				report.misses,
				fmt.Sprintf("%q -> ожидалось %s, получено %s",
					truncateForLog(testCase.text), testCase.want, firstFew(ranked)),
			)
		default:
			if position < 1 {
				hits1++
			}
			if position < 3 {
				hits3++
			}
			if position < 10 {
				hits10++
			}
		}
	}
	if report.total > 0 {
		report.at1 = float64(hits1) / float64(report.total)
		report.at3 = float64(hits3) / float64(report.total)
		report.at10 = float64(hits10) / float64(report.total)
	}
	return report
}

// rankedUnitIDs разворачивает closed-union результата в плоский порядок,
// в котором его увидит агент. Порядок ролей не означает применимость; здесь он
// используется только как наблюдаемая последовательность чтения.
func rankedUnitIDs(result fpf.QueryResult) []string {
	switch typed := result.(type) {
	case fpf.ExactHit:
		return []string{typed.Unit.UnitID}
	case fpf.CandidateSet:
		ordered := make([]string, 0, 16)
		for _, group := range typed.Groups {
			for _, candidate := range group.Candidates {
				ordered = append(ordered, candidate.Source.UnitID)
			}
		}
		return ordered
	default:
		return nil
	}
}

var evalWordPattern = regexp.MustCompile(`[\p{L}\p{N}]+`)

// buildDegradedAuthoredPhraseCases берёт авторские фразы и вычитает из них
// слова заголовка целевой единицы. Это механическая, воспроизводимая
// деградация: она убирает ровно те слова, которые делают попадание
// тривиальным, и оставляет вопрос о том, переживает ли поиск смену словаря.
func buildDegradedAuthoredPhraseCases(
	t *testing.T,
	db *sql.DB,
	limit int,
) []fpfRecallCase {
	t.Helper()
	rows, err := db.Query(`
		SELECT p.unit_id, p.phrase, u.title
		FROM source_authored_phrases p
		JOIN source_units u ON u.unit_id = p.unit_id
		WHERE LENGTH(p.phrase) > 40
		ORDER BY p.unit_id, p.phrase`)
	if err != nil {
		t.Fatalf("load authored phrases: %v", err)
	}
	defer func() { _ = rows.Close() }()

	cases := make([]fpfRecallCase, 0, limit)
	for rows.Next() {
		var unitID, phrase, title string
		if err := rows.Scan(&unitID, &phrase, &title); err != nil {
			t.Fatalf("scan authored phrase: %v", err)
		}
		titleWords := map[string]bool{}
		for _, word := range evalWordPattern.FindAllString(strings.ToLower(title), -1) {
			titleWords[word] = true
		}
		kept := make([]string, 0, 12)
		for _, word := range evalWordPattern.FindAllString(strings.ToLower(phrase), -1) {
			if len(word) < 4 || titleWords[word] {
				continue
			}
			kept = append(kept, word)
		}
		if len(kept) < 4 {
			continue
		}
		if len(kept) > 12 {
			kept = kept[:12]
		}
		cases = append(cases, fpfRecallCase{
			text: strings.Join(kept, " "),
			want: unitID,
		})
		if len(cases) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate authored phrases: %v", err)
	}
	sort.Slice(cases, func(left, right int) bool {
		return cases[left].text < cases[right].text
	})
	return cases
}

func truncateForLog(value string) string {
	if len(value) <= 58 {
		return value
	}
	return value[:58] + "…"
}

func firstFew(ranked []string) string {
	if len(ranked) == 0 {
		return "ничего"
	}
	if len(ranked) > 3 {
		ranked = ranked[:3]
	}
	return strings.Join(ranked, ", ")
}
