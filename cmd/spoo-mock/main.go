// Command spoo-mock is a local fake of the spoo.me API that serves rich,
// consistent, screenshot-ready data to every spoo CLI command.
//
// Usage:
//
//	go run ./cmd/spoo-mock            # serves on :8080
//	export SPOO_API_URL=http://localhost:8080
//	echo spoo_demo | spoo auth login --with-token
//	spoo stats        # full dashboard, ~287k clicks
//	spoo links        # long, paginated link browser
//	spoo whoami
//
// All data is deterministic (values keyed off labels/dates, not the clock),
// so screenshots are reproducible run to run. Dates are relative to "now"
// so the activity always looks recent.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	spoo "github.com/spoo-me/spoo-go"
)

// ── reference data ──────────────────────────────────────────────────────────

type weighted struct {
	label string
	w     float64
}

// West-heavy, with a light touch of Asia / South Asia.
var countries = []weighted{
	{"United States", 38}, {"United Kingdom", 15}, {"Germany", 11},
	{"Canada", 9}, {"France", 7.5}, {"Netherlands", 6}, {"Australia", 5},
	{"Spain", 4}, {"Sweden", 3.5}, {"Ireland", 3}, {"Italy", 2.6},
	{"Switzerland", 2.2}, {"Norway", 1.8}, {"Belgium", 1.6},
	{"India", 4.2}, {"Japan", 2.4}, {"Singapore", 1.5}, {"Brazil", 2},
}

var cities = []weighted{
	{"San Francisco", 14}, {"New York", 12}, {"London", 11.5}, {"Berlin", 7},
	{"Toronto", 6}, {"Amsterdam", 5.5}, {"Austin", 5}, {"Seattle", 4.8},
	{"Paris", 4.5}, {"Los Angeles", 4}, {"Chicago", 3.6}, {"Sydney", 3.4},
	{"Dublin", 3}, {"Stockholm", 2.8}, {"Boston", 2.6}, {"Munich", 2.4},
	{"Vancouver", 2.2}, {"Zurich", 2}, {"Manchester", 1.8},
	{"Bangalore", 2.6}, {"Tokyo", 2.1}, {"Singapore", 1.4},
}

var browsers = []weighted{
	{"Chrome", 46}, {"Safari", 24}, {"Firefox", 11}, {"Edge", 9},
	{"Brave", 5}, {"Arc", 3}, {"Opera", 1.4}, {"Samsung Internet", 0.9},
}

var oses = []weighted{
	{"macOS", 34}, {"Windows", 30}, {"iOS", 16}, {"Android", 11},
	{"Linux", 7}, {"ChromeOS", 2},
}

var referrers = []weighted{
	{"Direct", 27}, {"google.com", 18}, {"x.com", 12},
	{"news.ycombinator.com", 9}, {"github.com", 8}, {"reddit.com", 6},
	{"producthunt.com", 5}, {"linkedin.com", 4.5}, {"newsletter.spoo.me", 3.5},
	{"dev.to", 2.6}, {"duckduckgo.com", 2}, {"bing.com", 1.4},
}

var weekdays = []weighted{
	{"Monday", 17}, {"Tuesday", 18}, {"Wednesday", 18.5}, {"Thursday", 17.5},
	{"Friday", 14}, {"Saturday", 7.5}, {"Sunday", 7.5},
}

// linkSeed is the showcase link list: long, important-looking URLs with
// varied clicks, statuses and flags.
type linkSeed struct {
	alias, long       string
	clicks            int
	status            string
	pwd, bots, priv   bool
	maxClicks         int
	expireDays        int
	ageDays, lastDays int
}

var linkSeeds = []linkSeed{
	{"launch-hq", "https://spoo.me/blog/2026/introducing-spoo-cli-the-fastest-way-to-shorten-and-analyze-links-without-leaving-your-terminal", 51240, "ACTIVE", false, true, false, 0, 0, 78, 0},
	{"hn", "https://news.ycombinator.com/item?id=41928374", 41205, "ACTIVE", false, false, false, 0, 0, 9, 0},
	{"pricing", "https://spoo.me/pricing?utm_source=producthunt&utm_medium=launch&utm_campaign=spoo-cli-2026&ref=top-banner", 33118, "ACTIVE", false, true, false, 0, 0, 64, 0},
	{"ph", "https://www.producthunt.com/posts/spoo-cli?comment=the-official-command-line-client-for-spoo-me-is-finally-here", 27840, "ACTIVE", false, false, false, 0, 0, 12, 0},
	{"docs-api", "https://docs.spoo.me/api-reference/endpoints/create-short-url-with-password-expiry-and-bot-protection", 24317, "ACTIVE", false, false, false, 0, 0, 120, 1},
	{"gh-release", "https://github.com/spoo-me/spoo-cli/releases/tag/v0.1.1", 18793, "ACTIVE", false, false, false, 0, 0, 1, 0},
	{"demo", "https://www.youtube.com/watch?v=8aGhZQkoFbQ&list=PLspoomeDemos&index=2&t=94s&ab_channel=spoome", 15622, "ACTIVE", false, false, false, 0, 0, 30, 0},
	{"discord", "https://discord.com/invite/spoo-me-community-for-link-shorteners-and-analytics-nerds", 12451, "ACTIVE", false, false, false, 0, 0, 210, 0},
	{"changelog", "https://spoo.me/changelog#v0-1-1-remove-gated-custom-domains-and-ship-the-homebrew-cask", 9871, "ACTIVE", false, false, false, 0, 0, 2, 0},
	{"blackfriday", "https://spoo.me/promo/black-friday-2026-pro-plan-fifty-percent-off-the-first-year-limited-time-only", 8654, "INACTIVE", false, true, false, 0, 0, 165, 40},
	{"survey", "https://forms.gle/x7Qd9LmP2vK4nR8s-developer-experience-survey-2026-help-shape-the-roadmap", 7342, "ACTIVE", false, false, false, 0, 0, 48, 1},
	{"newsletter-42", "https://newsletter.spoo.me/archive/issue-42-shipping-the-cli-and-what-we-learned-about-the-oauth-device-flow", 6238, "ACTIVE", false, false, false, 0, 0, 7, 0},
	{"brew", "https://github.com/spoo-me/homebrew-tap/blob/main/Casks/spoo.rb", 5121, "ACTIVE", false, false, false, 0, 0, 1, 0},
	{"careers-pe", "https://spoo.me/careers/senior-platform-engineer-distributed-systems-remote-emea-or-north-america-2026", 4517, "ACTIVE", false, false, false, 0, 0, 53, 2},
	{"status", "https://status.spoo.me/incidents/2026-06-redis-failover-postmortem-and-the-mitigations-we-shipped", 3984, "ACTIVE", false, false, false, 0, 0, 5, 0},
	{"roadmap", "https://github.com/orgs/spoo-me/projects/3/views/1?filterQuery=milestone%3Av0.2-shell-completion-and-upgrade-nudges", 2872, "ACTIVE", false, false, false, 0, 0, 21, 0},
	{"webinar", "https://us02web.zoom.us/webinar/register/WN_self-hosting-spoo-me-on-kubernetes-with-helm-redis-and-mongodb", 2214, "ACTIVE", false, false, false, 5000, 0, 16, 1},
	{"beta", "https://spoo.me/beta/custom-domains-early-access-allowlist-signup-q3-2026-join-the-waitlist", 1923, "ACTIVE", false, false, true, 0, 0, 33, 0},
	{"press", "https://spoo.me/press/media-kit-logos-brand-guidelines-and-high-resolution-product-screenshots.zip", 1346, "ACTIVE", false, false, false, 0, 0, 41, 3},
	{"figma", "https://www.figma.com/file/9aZ2kLpQ/spoo-me-dashboard-redesign-2026?node-id=812%3A4471&t=demo-share", 1188, "ACTIVE", true, false, true, 0, 0, 26, 1},
	{"gist", "https://gist.github.com/spoo-me/3f9c1e7b2a4d6e8f0c5a7b9d1e3f5a7c-deploy-with-docker-compose-and-caddy", 1034, "ACTIVE", false, false, false, 0, 0, 38, 2},
	{"q3-okrs", "https://notion.so/spoome/Q3-2026-OKRs-Growth-Reliability-and-Developer-Experience-1a2b3c4d5e6f7890", 879, "ACTIVE", true, false, true, 0, 0, 35, 4},
	{"stripe", "https://dashboard.stripe.com/test/payment-links/plink_1QspoomeProAnnualUpgradeFlow2026demo", 742, "ACTIVE", true, false, false, 0, 0, 58, 6},
	{"twitter-thread", "https://x.com/spoo_me/status/1798342176590283471-we-rebuilt-the-cli-from-scratch-heres-what-changed", 688, "ACTIVE", false, false, false, 0, 0, 11, 0},
	{"sponsors", "https://github.com/sponsors/spoo-me?frequency=recurring&utm_campaign=readme-footer-cta", 611, "ACTIVE", false, false, false, 0, 0, 90, 5},
	{"calendly", "https://calendly.com/spoo-me/30min-intro-call-self-hosting-and-enterprise-link-management", 540, "ACTIVE", false, false, false, 0, 0, 44, 3},
	{"k8s-guide", "https://docs.spoo.me/self-hosting/kubernetes-deployment-with-horizontal-pod-autoscaling-and-redis-sentinel", 498, "ACTIVE", false, false, false, 0, 0, 62, 2},
	{"talk", "https://www.youtube.com/watch?v=qZ3kP8nL1aA&t=612s-building-a-url-shortener-that-scales-fosdem-2026", 451, "ACTIVE", false, false, false, 0, 0, 73, 4},
	{"raycast", "https://www.raycast.com/spoo-me/spoo-me-shorten-and-manage-links-without-leaving-your-keyboard", 402, "ACTIVE", false, false, false, 0, 0, 51, 1},
	{"swag", "https://spoo.me/store/limited-edition-launch-week-stickers-and-the-terminal-ghost-enamel-pin", 366, "ACTIVE", false, false, false, 2000, 0, 28, 2},
	{"intern", "https://spoo.me/careers/open-source-engineering-intern-summer-2026-remote-stipend-included", 318, "ACTIVE", false, false, false, 0, 0, 47, 7},
	{"postman", "https://www.postman.com/spoo-me/workspace/spoo-me-public-api/collection/url-shortening-and-stats", 274, "ACTIVE", false, false, false, 0, 90, 36, 3},
	{"og-preview", "https://spoo.me/tools/open-graph-preview-debugger-for-shortened-links-with-rich-card-rendering", 233, "ACTIVE", false, false, false, 0, 0, 19, 1},
	{"security", "https://spoo.me/.well-known/security.txt-responsible-disclosure-policy-and-our-bug-bounty-scope", 196, "ACTIVE", false, false, false, 0, 0, 88, 9},
	{"old-promo", "https://spoo.me/promo/spring-2026-early-adopter-discount-code-EARLYBIRD-now-expired-thanks-everyone", 142, "INACTIVE", false, false, false, 0, 0, 132, 70},
	{"qr-batch", "https://spoo.me/tools/bulk-qr-code-generator-export-as-svg-png-and-pdf-for-print-campaigns", 118, "ACTIVE", false, false, false, 0, 0, 24, 5},
	{"affiliate", "https://spoo.me/partners/affiliate-program-terms-and-thirty-percent-recurring-commission-2026", 97, "ACTIVE", true, false, true, 0, 0, 60, 11},
	{"draft", "https://spoo.me/blog/drafts/the-economics-of-running-a-free-url-shortener-at-scale-unpublished", 54, "INACTIVE", true, false, true, 0, 0, 6, 6},
	{"test-link", "https://example.com/a/very/deep/path/that/keeps/going/to/test/truncation/in/the/terminal/ui/nicely", 38, "ACTIVE", false, false, false, 100, 14, 3, 0},
	{"wip", "https://spoo.me/labs/experimental-ai-powered-alias-suggestions-based-on-page-title-and-content", 21, "ACTIVE", false, true, true, 0, 0, 4, 1},
}

var (
	demoUser  spoo.User
	demoLinks []spoo.URLItem
	now       = time.Now().UTC()
)

func init() {
	demoUser = spoo.User{
		ID:            "65f0a1b2c3d4e5f607182930",
		Email:         "demo@spoo.me",
		EmailVerified: true,
		Name:          "spoo demo",
		Plan:          "pro",
	}
	demoLinks = make([]spoo.URLItem, len(linkSeeds))
	for i, s := range linkSeeds {
		item := spoo.URLItem{
			ID:           fmt.Sprintf("65f0%020x", i+1),
			Alias:        s.alias,
			LongURL:      s.long,
			TotalClicks:  s.clicks,
			Status:       s.status,
			CreatedAt:    spoo.Timestamp{Time: now.AddDate(0, 0, -s.ageDays)},
			LastClick:    spoo.Timestamp{Time: now.AddDate(0, 0, -s.lastDays)},
			PasswordSet:  s.pwd,
			BlockBots:    s.bots,
			PrivateStats: s.priv,
		}
		if s.maxClicks > 0 {
			mc := s.maxClicks
			item.MaxClicks = &mc
		}
		if s.expireDays > 0 {
			item.ExpireAfter = spoo.Timestamp{Time: now.AddDate(0, 0, s.expireDays)}
		}
		demoLinks[i] = item
	}
}

// ── deterministic series + distributions ────────────────────────────────────

func hash(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

// uniqueRatio is a stable per-label unique/total ratio in [0.66, 0.80).
func uniqueRatio(label string) float64 {
	return 0.66 + float64(hash(label)%14)/100
}

// dayShape is a deterministic daily traffic multiplier: an upward trend
// toward today, weekday dips, and a few launch spikes.
func dayShape(d time.Time) float64 {
	daysAgo := int(now.Sub(d).Hours()/24 + 0.5)
	if daysAgo < 0 {
		daysAgo = 0
	}
	t := daysAgo
	if t > 90 {
		t = 90
	}
	trend := 1.0 + float64(90-t)/90*1.7
	wd := 1.0 // Tue/Wed/Thu
	switch d.Weekday() {
	case time.Saturday, time.Sunday:
		wd = 0.55
	case time.Monday, time.Friday:
		wd = 1.06
	}
	spike := 1.0
	switch daysAgo {
	case 1, 2:
		spike = 2.7 // release week
	case 9:
		spike = 2.1 // hit the HN front page
	case 12:
		spike = 1.8
	case 30:
		spike = 1.6
	}
	return trend * wd * spike
}

func timeSeries(start, end time.Time, total float64) (clicks, unique []map[string]any) {
	if end.Before(start) {
		start, end = end, start
	}
	days := int(end.Sub(start).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	if days > 120 {
		days = 120
	}
	shapes := make([]float64, days)
	var sum float64
	for i := range shapes {
		shapes[i] = dayShape(start.AddDate(0, 0, i))
		sum += shapes[i]
	}
	if sum == 0 {
		sum = 1
	}
	scale := total / sum
	for i := range shapes {
		label := start.AddDate(0, 0, i).Format("2006-01-02")
		c := math.Round(shapes[i] * scale)
		clicks = append(clicks, map[string]any{"time": label, "clicks": c})
		unique = append(unique, map[string]any{"time": label, "unique_clicks": math.Round(c * uniqueRatio(label))})
	}
	return clicks, unique
}

func distribute(dim string, items []weighted, total float64) (clicks, unique []map[string]any) {
	var wsum float64
	for _, it := range items {
		wsum += it.w
	}
	if wsum == 0 {
		wsum = 1
	}
	for _, it := range items {
		c := math.Round(it.w / wsum * total)
		clicks = append(clicks, map[string]any{dim: it.label, "clicks": c})
		unique = append(unique, map[string]any{dim: it.label, "unique_clicks": math.Round(c * uniqueRatio(it.label))})
	}
	return clicks, unique
}

// topLinkSeries builds the account-wide "top links" dimension straight
// from the link list, so the stats panel matches `spoo links`.
func topLinkSeries(n int) (clicks, unique []map[string]any) {
	ranked := append([]spoo.URLItem(nil), demoLinks...)
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].TotalClicks > ranked[j].TotalClicks })
	if n > len(ranked) {
		n = len(ranked)
	}
	for _, l := range ranked[:n] {
		c := float64(l.TotalClicks)
		clicks = append(clicks, map[string]any{"short_code": l.Alias, "clicks": c})
		unique = append(unique, map[string]any{"short_code": l.Alias, "unique_clicks": math.Round(c * uniqueRatio(l.Alias))})
	}
	return clicks, unique
}

func buildMetrics(start, end time.Time, total float64, perLink bool) map[string][]map[string]any {
	m := map[string][]map[string]any{}
	m["clicks_by_time"], m["unique_clicks_by_time"] = timeSeries(start, end, total)
	for dim, items := range map[string][]weighted{
		"browser": browsers, "os": oses, "country": countries,
		"city": cities, "referrer": referrers, "weekday": weekdays,
	} {
		m["clicks_by_"+dim], m["unique_clicks_by_"+dim] = distribute(dim, items, total)
	}
	if !perLink {
		m["clicks_by_short_code"], m["unique_clicks_by_short_code"] = topLinkSeries(12)
	}
	return m
}

// ── handlers ────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func parseRange(q map[string][]string) (start, end time.Time) {
	end = now
	start = now.AddDate(0, 0, -spoo.MaxRangeDays)
	if v := first(q, "start_date"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			start = t
		}
	}
	if v := first(q, "end_date"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			end = t
		}
	}
	return start, end
}

func first(q map[string][]string, key string) string {
	if vs := q[key]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// shapeSum integrates dayShape over [start, end] so range totals scale
// with the traffic curve — a previous-period window lands lower on the
// upward trend and the dashboard's period-over-period delta reads real.
func shapeSum(start, end time.Time) float64 {
	if end.Before(start) {
		start, end = end, start
	}
	days := int(end.Sub(start).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	if days > 120 {
		days = 120
	}
	var sum float64
	for i := 0; i < days; i++ {
		sum += dayShape(start.AddDate(0, 0, i))
	}
	return sum
}

// linkByAlias finds a demo link by its alias; nil when unknown.
func linkByAlias(alias string) *spoo.URLItem {
	for i := range demoLinks {
		if demoLinks[i].Alias == alias {
			return &demoLinks[i]
		}
	}
	return nil
}

// linkByID finds a demo link by its url id; nil when unknown.
func linkByID(id string) *spoo.URLItem {
	for i := range demoLinks {
		if demoLinks[i].ID == id {
			return &demoLinks[i]
		}
	}
	return nil
}

func notFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"error":"URL not found","code":"not_found"}`))
}

// statsHandler serves the account-wide GET /api/v1/stats; short_code
// arrives only as a plain filter (drill-down), never as a target.
func statsHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	code := first(q, "short_code")

	total := 0.0
	for _, l := range demoLinks {
		total += float64(l.TotalClicks)
	}
	if code != "" {
		total = 1200 // default if the filter value is unknown
		if l := linkByAlias(code); l != nil {
			total = float64(l.TotalClicks)
		}
	}
	writeJSON(w, statsResponse(q, total, code != "", nil))
}

// linkStatsHandler serves the per-link GET /api/v1/stats/links/{id}.
func linkStatsHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/stats/links/")
	l := linkByID(id)
	if l == nil {
		notFound(w)
		return
	}
	writeJSON(w, statsResponse(r.URL.Query(), float64(l.TotalClicks), true, l))
}

// demoLinkPassword unlocks every password-protected demo link.
const demoLinkPassword = "hunter2"

// publicStatsHandler serves GET/POST /api/v1/public/stats/{code}: the
// same stats wire inside the {generation, link, stats} envelope.
// Password-protected links answer 401 like production does — the
// password travels in a POST body only, never the query string.
func publicStatsHandler(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/api/v1/public/stats/")
	l := linkByAlias(code)
	if l == nil || l.PrivateStats {
		notFound(w)
		return
	}
	if l.PasswordSet {
		var body struct {
			Password string `json:"password"`
		}
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		if body.Password != demoLinkPassword {
			errCode := "password_required"
			if body.Password != "" {
				errCode = "invalid_password"
			}
			w.Header().Set("X-Error-Code", errCode)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, `{"error":"Password required","code":%q}`, errCode)
			return
		}
	}
	writeJSON(w, map[string]any{
		"generation": "v2",
		"link": map[string]any{
			"alias":              l.Alias,
			"short_url":          "https://spoo.me/" + l.Alias,
			"long_url":           l.LongURL,
			"created_at":         l.CreatedAt,
			"status":             strings.ToLower(l.Status),
			"password_protected": l.PasswordSet,
			"block_bots":         l.BlockBots,
		},
		"stats": statsResponse(r.URL.Query(), float64(l.TotalClicks), true, l),
	})
}

// statsResponse builds the standard stats wire for a date range and
// total; perLink drops the top-links dimension, and link (when set)
// echoes url_id and alias the way the per-link endpoints do.
func statsResponse(q map[string][]string, total float64, perLink bool, link *spoo.URLItem) spoo.StatsResponse {
	start, end := parseRange(q)
	if base := shapeSum(now.AddDate(0, 0, -spoo.MaxRangeDays), now); base > 0 {
		total = math.Round(total * shapeSum(start, end) / base)
	}

	uniqueTotal := math.Round(total * 0.713)
	resp := spoo.StatsResponse{
		Summary: spoo.StatsSummary{
			TotalClicks:        int(total),
			UniqueClicks:       int(uniqueTotal),
			FirstClick:         spoo.Timestamp{Time: start},
			LastClick:          spoo.Timestamp{Time: now.Add(-37 * time.Minute)},
			AvgRedirectionTime: 28.4,
		},
		TimeRange: spoo.StatsTimeRange{
			StartDate: spoo.Timestamp{Time: start},
			EndDate:   spoo.Timestamp{Time: end},
		},
		Metrics: buildMetrics(start, end, total, perLink),
		ComputedMetrics: map[string]float64{
			"unique_click_rate":          71.3,
			"repeat_click_rate":          28.7,
			"average_clicks_per_visitor": 1.4,
		},
		GeneratedAt: spoo.Timestamp{Time: now},
	}
	if link != nil {
		resp.URLID, resp.Alias = link.ID, link.Alias
	}
	return resp
}

func urlsHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items := append([]spoo.URLItem(nil), demoLinks...)

	// filter (search + status come through the `filter` JSON blob)
	if blob := first(q, "filter"); blob != "" {
		var f struct {
			Search string `json:"search"`
			Status string `json:"status"`
		}
		_ = json.Unmarshal([]byte(blob), &f)
		if f.Search != "" {
			needle := strings.ToLower(f.Search)
			kept := items[:0]
			for _, l := range items {
				if strings.Contains(strings.ToLower(l.Alias), needle) ||
					strings.Contains(strings.ToLower(l.LongURL), needle) {
					kept = append(kept, l)
				}
			}
			items = kept
		}
		if f.Status != "" {
			kept := items[:0]
			for _, l := range items {
				if l.Status == f.Status {
					kept = append(kept, l)
				}
			}
			items = kept
		}
	}

	// sort
	desc := first(q, "sortOrder") != "ascending"
	switch first(q, "sortBy") {
	case "created_at":
		sort.SliceStable(items, func(i, j int) bool { return less(items[i].CreatedAt.After(items[j].CreatedAt.Time), desc) })
	case "last_click":
		sort.SliceStable(items, func(i, j int) bool { return less(items[i].LastClick.After(items[j].LastClick.Time), desc) })
	default: // total_clicks
		sort.SliceStable(items, func(i, j int) bool { return less(items[i].TotalClicks > items[j].TotalClicks, desc) })
	}

	total := len(items)
	page := atoiOr(first(q, "page"), 1)
	size := atoiOr(first(q, "pageSize"), 20)
	if size <= 0 {
		size = 20
	}
	start := (page - 1) * size
	if start < 0 || start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	writeJSON(w, spoo.URLPage{
		Items:    items[start:end],
		Page:     page,
		PageSize: size,
		Total:    total,
		HasNext:  end < total,
	})
}

func less(naturalDesc, desc bool) bool {
	if desc {
		return naturalDesc
	}
	return !naturalDesc
}

func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

// ── auth ────────────────────────────────────────────────────────────────────

// deviceSession is the mock's single device-flow login: one live JWT
// pair, rotated on every refresh. Presenting a rotated-out refresh
// token answers 401 — exactly how a real expired session presents.
type deviceSession struct {
	mu      sync.Mutex
	n       int
	access  string
	refresh string
}

func (s *deviceSession) issue() (access, refresh string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	s.access = fmt.Sprintf("at-%d", s.n)
	s.refresh = fmt.Sprintf("rt-%d", s.n)
	return s.access, s.refresh
}

func (s *deviceSession) rotate(refresh string) (string, string, bool) {
	s.mu.Lock()
	if s.refresh == "" || refresh != s.refresh {
		s.mu.Unlock()
		return "", "", false
	}
	s.mu.Unlock()
	access, next := s.issue()
	return access, next, true
}

func (s *deviceSession) validAccess(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.access != "" && token == s.access
}

var device deviceSession

// authorized accepts any spoo_ API key (the mock is single-user) or
// the device session's current access token. A stale access token is
// rejected, which is what drives the CLI's refresh path end to end.
func authorized(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	bearer, ok := strings.CutPrefix(auth, "Bearer ")
	if !ok || bearer == "" {
		return false
	}
	if strings.HasPrefix(bearer, "spoo_") {
		return true
	}
	return device.validAccess(bearer)
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"authentication required","code":"AUTHENTICATION_ERROR"}`))
}

// requireAuth gates the owner endpoints the way production does.
func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r) {
			unauthorized(w)
			return
		}
		next(w, r)
	}
}

// claimTokens holds the outstanding anonymous-creation proofs:
// url id → one-time token, burned on claim.
var (
	claimMu     sync.Mutex
	claimTokens = map[string]string{}
	claimedIDs  = map[string]bool{}
)

// ── handlers wired in main ──────────────────────────────────────────────────

func deviceTokenHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AppID        string `json:"app_id"`
		Code         string `json:"code"`
		CodeVerifier string `json:"code_verifier"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.AppID != "spoo-cli" || body.Code == "" || body.CodeVerifier == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid device code exchange","code":"VALIDATION_ERROR"}`))
		return
	}
	access, refresh := device.issue()
	writeJSON(w, map[string]any{
		"access_token": access, "refresh_token": refresh, "user": demoUser,
	})
}

func deviceRefreshHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	access, refresh, ok := device.rotate(body.RefreshToken)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid refresh token","code":"AUTHENTICATION_ERROR"}`))
		return
	}
	writeJSON(w, map[string]any{"access_token": access, "refresh_token": refresh})
}

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	var req spoo.ShortenRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	alias := req.Alias
	if alias == "" {
		alias = fmt.Sprintf("g%05x", hash(req.LongURL)%0xfffff)
	}
	res := spoo.ShortURL{
		ID:       fmt.Sprintf("65f1%020x", hash(alias)),
		ShortURL: "https://spoo.me/" + alias, Alias: alias,
		LongURL: req.LongURL, CreatedAt: spoo.Timestamp{Time: now}, Status: "ACTIVE",
	}
	if authorized(r) {
		res.OwnerID = demoUser.ID
	} else {
		// anonymous creations carry a one-time claim token
		res.ClaimToken = fmt.Sprintf("claim-%08x", hash(alias+req.LongURL))
		claimMu.Lock()
		claimTokens[res.ID] = res.ClaimToken
		claimMu.Unlock()
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, res)
}

// claimHandler serves POST /api/v1/urls/claim: {url_id, token} pairs,
// max 16, resolved independently — per-item outcomes, never a batch
// failure.
func claimHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Claims []struct {
			URLID string `json:"url_id"`
			Token string `json:"token"`
		} `json:"claims"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if len(body.Claims) == 0 || len(body.Claims) > 16 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"claims must contain 1 to 16 items","code":"VALIDATION_ERROR","field":"claims"}`))
		return
	}
	claimMu.Lock()
	defer claimMu.Unlock()
	claimed := 0
	results := make([]map[string]string, 0, len(body.Claims))
	for _, c := range body.Claims {
		status := "invalid"
		switch {
		case claimTokens[c.URLID] != "" && claimTokens[c.URLID] == c.Token:
			delete(claimTokens, c.URLID) // token burns on claim
			claimedIDs[c.URLID] = true
			status = "claimed"
			claimed++
		case claimedIDs[c.URLID]:
			status = "already_yours"
		}
		results = append(results, map[string]string{"url_id": c.URLID, "status": status})
	}
	writeJSON(w, map[string]any{"results": results, "claimed": claimed})
}

// exportHandler serves the unified GET /api/v1/export. A url_id param
// slices the export to one link; an unknown id yields an empty file,
// not a 404 — consistent with the slicing filters.
func exportHandler(w http.ResponseWriter, r *http.Request) {
	format := first(r.URL.Query(), "format")
	if format == "" {
		format = "json"
	}
	ext := format
	if format == "csv" {
		ext = "zip"
	}
	name := "spoo-demo-export." + ext
	payload := `{"export":"demo","rows":2840,"generated_at":"` + now.Format(time.RFC3339) + `"}`
	if id := first(r.URL.Query(), "url_id"); id != "" {
		if l := linkByID(id); l != nil {
			name = "spoo-" + l.Alias + "-export." + ext
			payload = fmt.Sprintf(`{"export":"demo","alias":%q,"rows":%d}`, l.Alias, l.TotalClicks)
		} else {
			payload = "" // unknown id: an empty slice of the export
		}
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(payload))
}

// urlHandler serves the /api/v1/urls/ subtree: GET {id}, GET
// {domain}/{alias} (resolve), PATCH {id}, PATCH {id}/status, DELETE
// {id}, and POST claim.
func urlHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/urls/")
	segs := strings.Split(rest, "/")
	if rest == "claim" && r.Method == http.MethodPost {
		claimHandler(w, r)
		return
	}
	switch {
	case r.Method == http.MethodGet && len(segs) == 2:
		// GET {domain}/{alias} resolves a link to its id
		if alias, err := url.PathUnescape(segs[1]); err == nil {
			if l := linkByAlias(alias); l != nil {
				writeJSON(w, l)
				return
			}
		}
		notFound(w)
	case r.Method == http.MethodGet && len(segs) == 1:
		if l := linkByID(segs[0]); l != nil {
			writeJSON(w, l)
			return
		}
		notFound(w)
	case r.Method == http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPatch && len(segs) == 2 && segs[1] == "status":
		var body struct {
			Status string `json:"status"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if l := linkByID(segs[0]); l != nil {
			l.Status = body.Status // the TUI refetches; keep the list honest
		}
		writeJSON(w, map[string]any{"id": segs[0], "status": body.Status, "updated_at": now.Unix()})
	default:
		// generic PATCH: succeed so the TUI flows work for screenshots
		writeJSON(w, map[string]any{"id": segs[0], "status": "ACTIVE", "updated_at": now.Unix()})
	}
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/stats", requireAuth(statsHandler))
	mux.HandleFunc("/api/v1/stats/links/", requireAuth(linkStatsHandler))
	mux.HandleFunc("/api/v1/public/stats/", publicStatsHandler)
	mux.HandleFunc("/api/v1/urls", requireAuth(urlsHandler))
	mux.HandleFunc("/api/v1/urls/", requireAuth(urlHandler))
	mux.HandleFunc("/api/v1/export", requireAuth(exportHandler))
	mux.HandleFunc("/auth/me", requireAuth(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"user": demoUser})
	}))
	mux.HandleFunc("/auth/device/token", deviceTokenHandler)
	mux.HandleFunc("/auth/device/refresh", deviceRefreshHandler)
	mux.HandleFunc("/api/v1/shorten/check-alias", func(w http.ResponseWriter, r *http.Request) {
		alias := r.URL.Query().Get("alias")
		taken := alias == "docs" || alias == "pricing" || alias == "ph"
		reason := ""
		if taken {
			reason = "alias already in use"
		}
		writeJSON(w, spoo.AliasCheck{Available: !taken, Reason: reason})
	})
	mux.HandleFunc("/api/v1/shorten", shortenHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// serve the redirect edge too, so `spoo inspect` and `spoo open`
		// behave: /{alias} answers 302 to the destination
		if alias := strings.TrimPrefix(r.URL.Path, "/"); alias != "" {
			if l := linkByAlias(alias); l != nil {
				http.Redirect(w, r, l.LongURL, http.StatusFound)
				return
			}
			notFound(w)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	fmt.Fprintf(os.Stderr, "spoo-mock serving rich demo data on http://localhost%s\n", addr)
	fmt.Fprintf(os.Stderr, "  export SPOO_API_URL=http://localhost%s && echo spoo_demo | spoo auth login --with-token\n", addr)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, "spoo-mock:", err)
		os.Exit(1)
	}
}
