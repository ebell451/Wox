package system

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"wox/plugin"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
)

var webSearchesSettingKey = "webSearches"

var webSearchIcon = plugin.NewWoxImageSvg(`<svg t="1700799533400" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="8846" width="200" height="200"><path d="M869.484748 1024a96.009331 96.009331 0 0 1-76.327418-37.923686l-174.736982-228.982254a96.009331 96.009331 0 0 1 152.654836-116.651337l174.736982 228.982254a96.009331 96.009331 0 0 1-76.327418 154.094976z" fill="#D34233" p-id="8847"></path><path d="M770.595138 640.92277a96.009331 96.009331 0 0 0-100.809798-34.563359 240.023327 240.023327 0 0 1-57.605598 65.766391c-3.360327 2.400233-7.2007 4.32042-11.041074 6.720653a96.009331 96.009331 0 0 0 16.801633 79.687745l70.566859 92.649004a432.041989 432.041989 0 0 0 39.843872-26.882612A429.161709 429.161709 0 0 0 826.760596 715.810048z" fill="#C1211A" p-id="8848"></path><path d="M490.727938 864.144464a432.041989 432.041989 0 1 1 261.625427-88.328584A432.041989 432.041989 0 0 1 490.727938 864.144464zM490.727938 192.079148a240.023327 240.023327 0 1 0 192.018662 96.009331 240.023327 240.023327 0 0 0-192.018662-96.009331z" fill="#F16A54" p-id="8849"></path></svg>`)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &WebSearchPlugin{})
}

type webSearch struct {
	Urls       []string
	Title      string
	Keyword    string
	IsFallback bool //if true, this search will be used when no other search is matched
	Icon       plugin.WoxImage
	Enabled    bool
}

type WebSearchPlugin struct {
	api         plugin.API
	webSearches []webSearch
}

func (r *WebSearchPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "c1e350a7-c521-4dc3-b4ff-509f720fde86",
		Name:          "WebSearch",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Provide the web search ability",
		Icon:          webSearchIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		SettingDefinitions: []definition.PluginSettingDefinitionItem{
			{
				Type:               definition.PluginSettingDefinitionTypeTable,
				IsPlatformSpecific: true,
				Value: &definition.PluginSettingValueTable{
					Key:           webSearchesSettingKey,
					Title:         "i18n:plugin_websearch_web_searches",
					SortColumnKey: "Keyword",
					SortOrder:     definition.PluginSettingValueTableSortOrderAsc,
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:          "Icon",
							Label:        "i18n:plugin_websearch_icon",
							Type:         definition.PluginSettingValueTableColumnTypeWoxImage,
							HideInUpdate: true,
							Width:        40,
						},
						{
							Key:     "Keyword",
							Label:   "i18n:plugin_websearch_trigger_keyword",
							Tooltip: "i18n:plugin_websearch_trigger_keyword_tooltip",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Validators: []validator.PluginSettingValidator{
								&validator.PluginSettingValidatorNotEmpty{},
							},
							Width: 60,
						},
						{
							Key:     "Title",
							Label:   "i18n:plugin_websearch_title",
							Tooltip: "i18n:plugin_websearch_title_tooltip",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Validators: []validator.PluginSettingValidator{
								&validator.PluginSettingValidatorNotEmpty{},
							},
						},
						{
							Key:         "Urls",
							Label:       "i18n:plugin_websearch_urls",
							Tooltip:     "i18n:plugin_websearch_urls_tooltip",
							HideInTable: true,
							Type:        definition.PluginSettingValueTableColumnTypeTextList,
							Validators: []validator.PluginSettingValidator{
								&validator.PluginSettingValidatorNotEmpty{},
							},
						},
						{
							Key:   "Enabled",
							Label: "i18n:plugin_websearch_enabled",
							Type:  definition.PluginSettingValueTableColumnTypeCheckbox,
							Width: 60,
						},
						{
							Key:     "IsFallback",
							Label:   "i18n:plugin_websearch_is_fallback",
							Tooltip: "i18n:plugin_websearch_is_fallback_tooltip",
							Type:    definition.PluginSettingValueTableColumnTypeCheckbox,
							Width:   60,
						},
					},
				},
			},
		},
	}
}

func (r *WebSearchPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API
	r.webSearches = r.loadWebSearches(ctx)
	r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("loaded %d web searches", len(r.webSearches)))

	r.api.OnSettingChanged(ctx, func(key string, value string) {
		if key == webSearchesSettingKey {
			r.indexIcons(ctx)
			r.webSearches = r.loadWebSearches(ctx)
		}
	})

	util.Go(ctx, "parse websearch icons", func() {
		r.indexIcons(ctx)
	})
}

func (r *WebSearchPlugin) indexIcons(ctx context.Context) {
	hasAnyIconIndexed := false
	for i, search := range r.webSearches {
		if search.Icon.ImageType == "" {
			r.webSearches[i].Icon = r.indexWebSearchIcon(ctx, search)
			hasAnyIconIndexed = true
		}
	}

	if hasAnyIconIndexed {
		marshal, err := json.Marshal(r.webSearches)
		if err == nil {
			r.api.SaveSetting(ctx, webSearchesSettingKey, string(marshal), false)
		}
	}
}

func (r *WebSearchPlugin) indexWebSearchIcon(ctx context.Context, search webSearch) plugin.WoxImage {
	//sort urls, so that we can get the same icon between different runs
	slices.Sort(search.Urls)

	img, err := getWebsiteIconWithCache(ctx, search.Urls[0])
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to get icon for %s: %s", search.Urls[0], err.Error()))
		return webSearchIcon
	}

	return img
}

func (r *WebSearchPlugin) loadWebSearches(ctx context.Context) (webSearches []webSearch) {
	webSearchesJson := r.api.GetSetting(ctx, webSearchesSettingKey)
	if webSearchesJson == "" {
		return
	}

	unmarshalErr := json.Unmarshal([]byte(webSearchesJson), &webSearches)
	if unmarshalErr != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to unmarshal web searches: %s", unmarshalErr.Error()))
		return
	}

	return
}

func (r *WebSearchPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	queries := strings.Split(query.RawQuery, " ")
	if len(queries) <= 1 {
		return
	}

	triggerKeyword := queries[0]
	otherQuery := strings.Join(queries[1:], " ")

	for _, search := range r.webSearches {
		if search.IsFallback || !search.Enabled {
			continue
		}
		if strings.ToLower(search.Keyword) == strings.ToLower(triggerKeyword) {
			results = append(results, plugin.QueryResult{Title: r.replaceVariables(ctx, search.Title, otherQuery),
				Score: 100,
				Icon:  search.Icon,
				Actions: []plugin.QueryResultAction{
					{
						Name: "Search",
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							for _, url := range search.Urls {
								util.ShellOpen(r.replaceVariables(ctx, url, otherQuery))
								time.Sleep(time.Millisecond * 100)
							}
						},
					},
				},
			})
		}
	}

	return
}

func (r *WebSearchPlugin) QueryFallback(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	for _, search := range r.webSearches {
		if !search.Enabled {
			continue
		}
		if !search.IsFallback {
			continue
		}

		results = append(results, plugin.QueryResult{
			Title: r.replaceVariables(ctx, search.Title, query.RawQuery),
			Score: 100,
			Icon:  search.Icon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "Search",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						for _, url := range search.Urls {
							util.ShellOpen(r.replaceVariables(ctx, url, query.RawQuery))
							time.Sleep(time.Millisecond * 100)
						}
					},
				},
			},
		})
	}

	return results
}

func (r *WebSearchPlugin) replaceVariables(ctx context.Context, text string, query string) string {
	result := strings.ReplaceAll(text, "{query}", query)
	result = strings.ReplaceAll(result, "{lower_query}", strings.ToLower(query))
	result = strings.ReplaceAll(result, "{upper_query}", strings.ToUpper(query))
	return result
}
