package setting

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
	"wox/util"
)

func TestUnMarshalPluginSettingItem(t *testing.T) {
	type metadataForTest struct {
		Settings CustomizedPluginSettings
	}

	jsonStr := `
{
    "Settings":[
        {
            "Type":"head",
            "Value":{
                "Content":"This is head title"
            }
        },
        {
            "Type":"textbox",
            "Value":{
                "Key":"IndexDirectories",
                "Value":"test;test1",
                "Label":"Index Directories: ",
                "Suffix":" (separate by ';')"
            }
        },
        {
            "Type":"checkbox",
            "Value":{
                "Key":"OnlyIndexTxt",
				"Value": "true",
                "Label":", Only Index Txt"
            }
        },
		{
					"Type":"select",
					"Value":{	
						"Key":"IndexPrograms",	
						"Value":"true",		
						"Label":"Index Programs: ",	
						"Options":[
							{"Label":"true", "Value":"true"},
							{"Label":"false", "Value":"false"}	
						]		
					}	
		},
        {
            "Type":"newline",
			"Value":{}
        },
        {
            "Type":"label",
            "Value":{
                "Content":"IndexPrograms"
            }
        }
    ]
}
`

	var metadata metadataForTest
	err := json.Unmarshal([]byte(jsonStr), &metadata)
	if err != nil {
		t.Log(err.Error())
	}

	assert.Nil(t, err)
	assert.Equal(t, len(metadata.Settings), 6)
	assert.Equal(t, metadata.Settings[0].Type, PluginSettingTypeHead)
	assert.Equal(t, metadata.Settings[1].Type, PluginSettingTypeTextBox)
	assert.Equal(t, metadata.Settings[2].Type, PluginSettingTypeCheckBox)
	assert.Equal(t, metadata.Settings[3].Type, PluginSettingTypeSelect)
	assert.Equal(t, metadata.Settings[4].Type, PluginSettingTypeNewLine)
	assert.Equal(t, metadata.Settings[5].Type, PluginSettingTypeLabel)
	assert.Equal(t, len(metadata.Settings[3].Value.(PluginSettingValueSelect).Options), 2)

	val, exist := metadata.Settings.GetValue("IndexDirectories")
	assert.True(t, exist)
	assert.Equal(t, val, "test;test1")

	marshalData, marshalErr := json.Marshal(metadata)
	assert.Nil(t, marshalErr)
	t.Log(string(marshalData))
}

func TestMarshalPluginSetting(t *testing.T) {
	var h util.HashMap[string, string]
	h.Store("test", "test")
	h.Store("test1", "test")

	ps := PluginSetting{
		Disabled:           true,
		TriggerKeywords:    nil,
		CustomizedSettings: &h,
	}

	marshalData, marshalErr := json.Marshal(ps)
	assert.Nil(t, marshalErr)
	t.Log(string(marshalData))

	var ps1 PluginSetting
	err := json.Unmarshal(marshalData, &ps1)
	assert.Nil(t, err)
	assert.Equal(t, ps.Disabled, ps1.Disabled)
	assert.Equal(t, ps1.CustomizedSettings.Len(), int64(2))
}
