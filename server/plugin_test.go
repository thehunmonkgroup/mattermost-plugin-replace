package main

import (
	"io/ioutil"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
	"github.com/mattermost/mattermost-server/plugin/plugintest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupTestPlugin(t *testing.T, api *plugintest.API) *Plugin {
	p := &Plugin{}

	p.SetAPI(api)

	return p
}

type testCase struct {
	message         string
	command         string
	rootId          string
	expectedMessage string
	isInvalidFormat bool
	shouldDismiss   bool
}

type testAPIConfig struct {
	User    *model.User
	Posts   []*model.Post
	Post    *model.Post
	Channel *model.Channel
}

func setupAPI(api *plugintest.API) {
	api.On("GetServerVersion").Return(minServerVersion)
}

// TestExecuteCommand mocks the API calls (by using the private method setupAPI) and validates the inputs given
func TestExecuteCommand(t *testing.T) {
	cases := []testCase{
		{"message to bee replaced", "s/bee/be", "", `message to be replaced`, false, true},
		{"message to bee replaced", "s/bee/be", "123", `message to be replaced`, false, true},
		{"message to bee replaced", " s/bee/be ", "", `message to be replaced`, false, true},
		{"baaad input", "s/bad", "", "", true, true},
		{"more baaad input", "s/baaad/", "", "", true, true},
		{"empty input", "s/", "", "", true, true},
		{"empty input", "s//", "", "", true, true},
		{"not a command", "hello world", "", "", false, false},
		{"contains s/ but not prefix", "this is not s/a/command", "", "", false, false},
		{"starts with s but not s/", "say s/hello/world", "", "", false, false},
		{"what if I typ the word typical", "s/typ/type", "", `what if I type the word typical`, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {

			c := &plugin.Context{}
			post := &model.Post{
				UserId:    "testUserId",
				Message:   tc.command,
				ChannelId: "testChannelId",
			}

			api := &plugintest.API{}

			defer api.AssertExpectations(t)

			config := &testAPIConfig{
				User:    &model.User{Id: post.UserId, Username: "test"},
				Posts:   []*model.Post{&model.Post{UserId: post.UserId, Message: tc.message}},
				Post:    &model.Post{},
				Channel: &model.Channel{TeamId: "testTeamId"},
			}

			setupAPI(api)

			p := setupTestPlugin(t, api)

			if !tc.isInvalidFormat && tc.shouldDismiss {
				api.On("GetUser", post.UserId).Return(config.User, nil)
				api.On("GetChannel", post.ChannelId).Return(config.Channel, nil)
				if tc.rootId == "" {
					api.On("SearchPostsInTeam", mock.AnythingOfType("string"), mock.AnythingOfType("[]*model.SearchParams")).Return(config.Posts, nil)
				} else {
					api.On("SearchPostsInTeam", mock.AnythingOfType("string"), mock.AnythingOfType("[]*model.SearchParams")).Return(config.Posts, nil)
				}
				api.On("UpdatePost", mock.AnythingOfType("*model.Post")).Return(config.Post, nil)
				api.On("SendEphemeralPost", post.UserId, mock.AnythingOfType("*model.Post")).Return(nil)
			} else if tc.isInvalidFormat && tc.shouldDismiss {
				api.On("SendEphemeralPost", post.UserId, mock.AnythingOfType("*model.Post")).Return(nil)
			}

			err := p.OnActivate()
			assert.Nil(t, err)

			returnedPost, returnedErr := p.MessageWillBePosted(c, post)

			assert.Nil(t, returnedPost)

			if tc.shouldDismiss {
				assert.Equal(t, "plugin.message_will_be_posted.dismiss_post", returnedErr)
			} else {
				assert.Equal(t, "", returnedErr)
			}

			api.AssertExpectations(t)
		})

		t.Run(tc.command+" - Replace", func(t *testing.T) {
			trimmedCmd := strings.TrimSpace(tc.command)
			oldAndNew, err := splitAndValidateInput(trimmedCmd)

			if tc.isInvalidFormat {
				assert.NotNil(t, err)
			} else if strings.HasPrefix(trimmedCmd, "s/") {
				assert.Nil(t, err)
				assert.NotNil(t, oldAndNew)
				assert.Len(t, oldAndNew, 2)
				if tc.expectedMessage != "" {
					assert.Equal(t, tc.expectedMessage, replace(tc.message, oldAndNew[0], oldAndNew[1]))
				}
			}
		})
	}
}

func TestPluginOnActivate(t *testing.T) {

	api := &plugintest.API{}

	api.On("GetServerVersion").Return(minServerVersion)

	defer api.AssertExpectations(t)

	p := setupTestPlugin(t, api)

	err := p.OnActivate()

	assert.Nil(t, err)
}

func TestServeHTTP(t *testing.T) {
	assert := assert.New(t)
	plugin := Plugin{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	plugin.ServeHTTP(nil, w, r)

	result := w.Result()
	assert.NotNil(result)
	bodyBytes, err := ioutil.ReadAll(result.Body)
	assert.Nil(err)
	bodyString := string(bodyBytes)

	assert.Equal("please log in\n", bodyString)
}
