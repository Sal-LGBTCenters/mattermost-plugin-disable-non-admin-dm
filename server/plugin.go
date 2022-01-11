package main

import (
	"strings"
	"sync"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pkg/errors"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration
}

func (p *Plugin) OnActivate() error {
	Mattermost = p.API

	if err := p.OnConfigurationChange(); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) MessageWillBePosted(c *plugin.Context, post *model.Post) (*model.Post, string) {
	conf := p.getConfiguration()

	channel, appError := Mattermost.GetChannel(post.ChannelId)

	if appError != nil {
		errors.Wrap(appError, "Failed to get channel for post: "+post.Id+" and channelId: "+post.ChannelId)
		return nil, ""
	}

	isUserTeamAdmin := func(userID string) bool {
		userteams, err := Mattermost.GetTeamsForUser(userID)
		if err != nil {
			return false
		}

		if len(userteams) < 1 {
			return false
		}

		for _, team := range userteams {
			tm, err := Mattermost.GetTeamMember(team.Id, userID)
			if err != nil {
				return false
			}
			if strings.Contains(tm.Roles, "team_admin") {
				return true
			}
		}
		return false
	}

	if isUserTeamAdmin(post.UserId) || isUserTeamAdmin(channel.GetOtherUserIdForDM(post.UserId)) {
		return nil, ""
	}

	if (channel.Type == model.ChannelTypeDirect && conf.RejectDMs) || (channel.Type == model.ChannelTypeGroup && conf.RejectGroupChats) {
		Mattermost.SendEphemeralPost(post.UserId, &model.Post{
			Message:   conf.RejectionMessage,
			ChannelId: post.ChannelId,
		})
		return nil, conf.RejectionMessage
	}

	return nil, ""
}

// See https://developers.mattermost.com/extend/plugins/server/reference/
