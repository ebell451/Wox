package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/selection"

	"github.com/google/uuid"
)

var aiChatIcon = plugin.PluginAIChatIcon
var aiChatsSettingKey = "ai_chats"

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &AIChatPlugin{})
}

type AIChatPlugin struct {
	chats           []common.AIChatData
	resultChatIdMap *util.HashMap[string /*chat id*/, string /*result id*/] // map of result id and chat id, used to update the chat title
	mcpServers      []AIChatMCPServerConfig
	mcpToolsMap     *util.HashMap[common.Tool, *AIChatMCPServerConfig]
	api             plugin.API
}

func (r *AIChatPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:              "a9cfd85a-6e53-415c-9d44-68777aa6323d",
		Name:            "AI Chat",
		Author:          "Wox Launcher",
		Website:         "https://github.com/Wox-launcher/Wox",
		Version:         "1.0.0",
		MinWoxVersion:   "2.0.0",
		Runtime:         "Go",
		Description:     "Chat with AI",
		Icon:            aiChatIcon.String(),
		TriggerKeywords: []string{"chat"},
		SupportedOS:     []string{"Windows", "Macos", "Linux"},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeSelectAIModel,
				Value: &definition.PluginSettingValueSelectAIModel{
					Key:     "default_model",
					Label:   "i18n:plugin_ai_chat_default_model",
					Tooltip: "i18n:plugin_ai_chat_default_model_tooltip",
					Style: definition.PluginSettingValueStyle{
						PaddingBottom: 8,
					},
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:     "mcp_servers",
					Title:   "i18n:plugin_ai_chat_mcp_servers",
					Tooltip: "i18n:plugin_ai_chat_mcp_servers_tooltip",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:     "name",
							Label:   "i18n:plugin_ai_chat_mcp_server_name",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   100,
							Tooltip: "i18n:plugin_ai_chat_mcp_server_name_tooltip",
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:     "type",
							Label:   "i18n:plugin_ai_chat_mcp_server_type",
							Type:    definition.PluginSettingValueTableColumnTypeSelect,
							Width:   100,
							Tooltip: "i18n:plugin_ai_chat_mcp_server_type_tooltip",
							SelectOptions: []definition.PluginSettingValueSelectOption{
								{
									Label: "STUDIO",
									Value: string(AIChatMCPServerTypeSTDIO),
								},
								{
									Label: "SSE",
									Value: string(AIChatMCPServerTypeSSE),
								},
							},
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:     "command",
							Label:   "i18n:plugin_ai_chat_mcp_server_command",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   80,
							Tooltip: "i18n:plugin_ai_chat_mcp_server_command_tooltip",
						},
						{
							Key:     "environmentVariables",
							Label:   "i18n:plugin_ai_chat_mcp_server_environment_variables",
							Type:    definition.PluginSettingValueTableColumnTypeTextList,
							Width:   100,
							Tooltip: "i18n:plugin_ai_chat_mcp_server_environment_variables_tooltip",
						},
						{
							Key:          "url",
							Label:        "i18n:plugin_ai_chat_mcp_server_url",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							TextMaxLines: 10,
							Width:        80,
							Tooltip:      "i18n:plugin_ai_chat_mcp_server_url_tooltip",
						},
					},
				},
			},
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
			{
				Name: plugin.MetadataFeatureAI,
			},
			{
				Name: plugin.MetadataFeatureResultPreviewWidthRatio,
				Params: map[string]string{
					"WidthRatio": "0.3",
				},
			},
		},
	}
}

func (r *AIChatPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.resultChatIdMap = util.NewHashMap[string, string]()
	r.mcpToolsMap = util.NewHashMap[common.Tool, *AIChatMCPServerConfig]()
	r.api = initParams.API
	r.mcpServers = []AIChatMCPServerConfig{}

	r.reloadMCPServers(ctx)
	r.api.OnSettingChanged(ctx, func(key string, value string) {
		if key == "mcp_servers" {
			r.reloadMCPServers(ctx)
		}
	})

	chats, err := r.loadChats(ctx)
	if err != nil {
		r.chats = []common.AIChatData{}
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to load chats: %s", err.Error()))
	} else {
		r.chats = chats
	}

}

func (r *AIChatPlugin) getDefaultModel() common.Model {
	model := r.api.GetSetting(context.Background(), "default_model")
	if model != "" {
		var m common.Model
		err := json.Unmarshal([]byte(model), &m)
		if err == nil {
			return m
		} else {
			r.api.Log(context.Background(), plugin.LogLevelError, fmt.Sprintf("Failed to unmarshal default model: %s", err.Error()))
		}
	}

	// get last chat model
	if len(r.chats) > 0 {
		lastChat := r.chats[0]
		return common.Model{
			Name:     lastChat.Model.Name,
			Provider: lastChat.Model.Provider,
		}
	}

	return common.Model{}
}

func (r *AIChatPlugin) reloadMCPServers(ctx context.Context) {
	mcpServers, err := r.loadMCPServers(ctx)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to load mcp servers: %s", err.Error()))
	} else {
		r.mcpServers = mcpServers
	}

	for _, mcpServer := range r.mcpServers {
		tools, err := mcpServer.listTool(ctx)
		if err != nil {
			r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to list tool: %s", err.Error()))
		}
		for _, tool := range tools {
			r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("%s tool %s: %s", mcpServer.Name, tool.Name, tool.Description))
			r.mcpToolsMap.Store(tool, &mcpServer)
		}
	}
}

func (r *AIChatPlugin) loadMCPServers(ctx context.Context) ([]AIChatMCPServerConfig, error) {
	mcpServersJson := r.api.GetSetting(ctx, "mcp_servers")
	if mcpServersJson == "" {
		return []AIChatMCPServerConfig{}, nil
	}

	var mcpServers []AIChatMCPServerConfig
	err := json.Unmarshal([]byte(mcpServersJson), &mcpServers)
	if err != nil {
		return []AIChatMCPServerConfig{}, err
	}

	return mcpServers, nil
}

func (r *AIChatPlugin) loadChats(ctx context.Context) ([]common.AIChatData, error) {
	chats := []common.AIChatData{}
	chatsJson := r.api.GetSetting(ctx, aiChatsSettingKey)
	if chatsJson == "" {
		return []common.AIChatData{}, nil
	}

	err := json.Unmarshal([]byte(chatsJson), &chats)
	if err != nil {
		return []common.AIChatData{}, err
	}

	sort.Slice(chats, func(i, j int) bool {
		return chats[i].UpdatedAt > chats[j].UpdatedAt
	})

	return chats, nil
}

func (r *AIChatPlugin) saveChats(ctx context.Context) {
	chatsJson, err := json.Marshal(r.chats)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to marshal chats: %s", err.Error()))
		return
	}

	r.api.SaveSetting(ctx, aiChatsSettingKey, string(chatsJson), false)
}

func (r *AIChatPlugin) Chat(ctx context.Context, aiChatData common.AIChatData) {
	// add a new conversation for AI response
	currentResponseConversationId := uuid.NewString()
	aiChatData.Conversations = append(aiChatData.Conversations, common.Conversation{
		Id:        currentResponseConversationId,
		Role:      common.ConversationRoleAI,
		Text:      "",
		Images:    []common.WoxImage{},
		Timestamp: util.GetSystemTimestamp(),
	})

	// find the chat by id
	found := false
	for i, chat := range r.chats {
		if chat.Id == aiChatData.Id {
			r.chats[i] = aiChatData
			found = true
			break
		}
	}
	// if not found, add it
	if !found {
		r.chats = append(r.chats, aiChatData)
		sort.Slice(r.chats, func(i, j int) bool {
			return r.chats[i].UpdatedAt > r.chats[j].UpdatedAt
		})
	}

	r.saveChats(ctx)

	// summarize the chat on 2th, 6th, 12th, 24th conversation
	summarizeIndex := []int{2, 6, 12, 24}
	for _, index := range summarizeIndex {
		if len(aiChatData.Conversations) == index {
			r.summarizeChat(ctx, aiChatData)
			break
		}
	}

	chatErr := r.api.AIChatStream(ctx, aiChatData.Model, aiChatData.Conversations, common.ChatOptions{
		Tools: r.mcpToolsMap.Keys(),
	}, func(t common.ChatStreamDataType, data string) {
		r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("chat stream data: %s", data))

		// find the aiResponseConversation and update
		var aiResponseConversation common.Conversation
		for _, conversation := range aiChatData.Conversations {
			if conversation.Id == currentResponseConversationId {
				aiResponseConversation = conversation
				break
			}
		}
		if aiResponseConversation.Id == "" {
			r.api.Log(ctx, plugin.LogLevelError, "current AI response conversation not found")
			return
		}

		var responseText string = aiResponseConversation.Text
		if t == common.ChatStreamTypeStreaming {
			responseText += data
			aiResponseConversation.Text = responseText
		} else if t == common.ChatStreamTypeFinished {
			responseText += data
			aiResponseConversation.Text = responseText
		} else if t == common.ChatStreamTypeError {
			responseText = "Error"
			aiResponseConversation.Text = responseText
		}

		// update the aiResponseConversation
		for i := range aiChatData.Conversations {
			if aiChatData.Conversations[i].Id == currentResponseConversationId {
				aiChatData.Conversations[i].Text = responseText
				break
			}
		}

		// send the chat response to UI
		plugin.GetPluginManager().GetUI().SendChatResponse(ctx, aiChatData)
	})

	if chatErr != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to chat: %s", chatErr.Error()))
		r.api.Notify(ctx, "Failed to chat, please try again")
	}
}

func (r *AIChatPlugin) getNewChatPreviewData(ctx context.Context) plugin.QueryResult {
	var chatData common.AIChatData
	chatData.Id = uuid.NewString()
	chatData.Title = ""
	chatData.CreatedAt = util.GetSystemTimestamp()
	chatData.UpdatedAt = util.GetSystemTimestamp()
	chatData.Conversations = []common.Conversation{}
	chatData.Model = r.getDefaultModel()

	previewData, err := json.Marshal(chatData)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to marshal chat preview data: %s", err.Error()))
		return plugin.QueryResult{}
	}

	resultId := uuid.NewString()
	r.resultChatIdMap.Store(chatData.Id, resultId)

	return plugin.QueryResult{
		Id:          resultId,
		Title:       "New Chat",
		SubTitle:    "Create a new chat",
		Icon:        aiChatIcon,
		ContextData: chatData.Id,
		Preview: plugin.WoxPreview{
			PreviewType:    plugin.WoxPreviewTypeChat,
			PreviewData:    string(previewData),
			ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
		},
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "Start Chat",
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					plugin.GetPluginManager().GetUI().FocusToChatInput(ctx)
				},
			},
		},
		Group:      "New Chat",
		GroupScore: 1000,
	}
}

func (r *AIChatPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	r.resultChatIdMap.Clear()

	// add the new chat result for user to create a new chat
	results = append(results, r.getNewChatPreviewData(ctx))

	for i, chat := range r.chats {
		previewData, err := json.Marshal(chat)
		if err != nil {
			r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to marshal chat preview data: %s", err.Error()))
			continue
		}

		resultId := uuid.NewString()
		r.resultChatIdMap.Store(chat.Id, resultId)

		continueChatText := "Continue Chat"
		if len(chat.Conversations) == 0 {
			continueChatText = "Start Chat"
		}

		group, groupScore := r.getResultGroup(ctx, chat)
		results = append(results, plugin.QueryResult{
			Id:          resultId,
			Title:       chat.Title,
			Icon:        aiChatIcon,
			ContextData: chat.Id,
			Preview: plugin.WoxPreview{
				PreviewType:    plugin.WoxPreviewTypeChat,
				PreviewData:    string(previewData),
				ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
			},
			Actions: []plugin.QueryResultAction{
				{
					Name:                   continueChatText,
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						// focus to chat input
						plugin.GetPluginManager().GetUI().FocusToChatInput(ctx)
					},
				},
				{
					Name:                   "Delete Chat",
					Icon:                   plugin.TrashIcon,
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						// delete chat
						r.chats = append(r.chats[:i], r.chats[i+1:]...)
						r.saveChats(ctx)

						// refresh the query results
						r.api.ChangeQuery(ctx, common.PlainQuery{
							QueryType:      plugin.QueryTypeInput,
							QueryText:      query.RawQuery,
							QuerySelection: selection.Selection{},
						})
					},
				},
				{
					Name:                   "Summarize Chat",
					Icon:                   common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="currentColor" d="M5 5.5C5 6.33 5.67 7 6.5 7h4v10.5c0 .83.67 1.5 1.5 1.5s1.5-.67 1.5-1.5V7h4c.83 0 1.5-.67 1.5-1.5S18.33 4 17.5 4h-11C5.67 4 5 4.67 5 5.5"/></svg>`),
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						chatId := actionContext.ContextData
						for _, chat := range r.chats {
							if chat.Id == chatId {
								r.summarizeChat(ctx, chat)
								break
							}
						}
					},
				},
			},
			Group:      group,
			GroupScore: groupScore,
		})
	}

	return results
}

func (r *AIChatPlugin) summarizeChat(ctx context.Context, chat common.AIChatData) {
	r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Summarizing chat: %s", chat.Id))

	conversations := chat.Conversations
	conversations = append(conversations, common.Conversation{
		Id:   uuid.NewString(),
		Role: common.ConversationRoleUser,
		Text: `Please summarize our conversation above and provide a clear and concise title. Requirements:
		1. The title should be no more than 10 characters. 
		2. The language of the title should be the same as the language of the conversation.
		3. The title should be a single sentence.
		4. The response should be only the title, no other text.
`,
		Images:    []common.WoxImage{},
		Timestamp: util.GetSystemTimestamp(),
	})

	title := ""
	r.api.AIChatStream(ctx, chat.Model, conversations, common.EmptyChatOptions, func(t common.ChatStreamDataType, data string) {
		r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("chat stream data: %s", data))
		if t == common.ChatStreamTypeStreaming {
			title += data
		} else if t == common.ChatStreamTypeFinished {
			title += data
			r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Summarized chat title: %s", title))

			// update the chat title
			for i := range r.chats {
				if r.chats[i].Id == chat.Id {
					r.chats[i].Title = title
					break
				}
			}
			r.saveChats(ctx)

			if resultId, ok := r.resultChatIdMap.Load(chat.Id); ok {
				plugin.GetPluginManager().GetUI().UpdateResult(ctx, common.UpdateableResult{
					Id:    resultId,
					Title: &title,
				})
			}
		}
	})
}

func (c *AIChatPlugin) getResultGroup(ctx context.Context, chat common.AIChatData) (string, int64) {
	if util.GetSystemTimestamp()-chat.UpdatedAt < 1000*60*60*24 {
		return "Today", 90
	}
	if util.GetSystemTimestamp()-chat.UpdatedAt < 1000*60*60*24*2 {
		return "Yesterday", 80
	}

	return "History", 10
}
