package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	irc "github.com/thoj/go-ircevent"
)

var config = flag.String("config", "", "configuration file")

type Config struct {

	// actual config
	Irc struct {
		Ssl           bool     `json:"ssl"`
		SslVerifySkip bool     `json:"ssl_verify_skip"`
		Port          string   `json:"port"`
		Nickname      string   `json:"nickname"`
		Channels      []string `json:"channels"`
		Host          string   `json:"host"`
		Password      string   `json:"password"`
		Ignore        []string `json:"ignore"`
	} `json:"irc"`
	Github struct {
		Token    string   `json:"token"`
		Projects []string `json:"projects"`
	} `json:"github"`

	// internal/caching stuff
	ProjectsByRepoName map[string]string
}

func (c *Config) Load(filename string) error {
	data, err := ioutil.ReadFile(filename)

	// I/O error
	if err != nil {
		return err
	}

	// JSON error
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}

	// validate config

	if c.Irc.Nickname == "" {
		c.Irc.Nickname = "issuebot"
	}

	if c.Irc.Host == "" {
		return errors.New("host is required")
	}

	if c.Github.Token == "" {
		return errors.New("token is required")
	}

	if len(c.Github.Projects) == 0 {
		return errors.New("projects is required")
	}

	for _, proj := range c.Github.Projects {
		if strings.IndexByte(proj, '/') == -1 {
			return errors.New("projects must be in format 'owner/repo'")
		}

		// map repo names to owners
		repo := strings.SplitN(proj, "/", 2)[1]
		c.ProjectsByRepoName[strings.ToLower(repo)] = proj
	}

	return nil
}

func main() {
	flag.Parse()
	c := &Config{ProjectsByRepoName: make(map[string]string)}

	if err := c.Load(*config); err != nil {
		log.Fatal(err)
	}

	ircproj := irc.IRC(c.Irc.Nickname, c.Irc.Nickname)
	ircproj.UseTLS = c.Irc.Ssl
	if c.Irc.SslVerifySkip {
		ircproj.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}
	ircproj.Password = c.Irc.Password

	err := ircproj.Connect(net.JoinHostPort(c.Irc.Host, c.Irc.Port))
	if err != nil {
		log.Fatal(err)
	}

	ircproj.AddCallback("001", func(event *irc.Event) {
		for _, channel := range c.Irc.Channels {
			ircproj.Join(channel)
		}
	})

	r := regexp.MustCompile(`([\w/]+)#(\d+)`)
	ircproj.AddCallback("PRIVMSG", func(event *irc.Event) {
		for _, ignoreNick := range c.Irc.Ignore {
			if event.Nick == ignoreNick || event.User == ignoreNick {
				return
			}
		}
		matches := r.FindAllStringSubmatch(event.Message(), 1)
		for _, match := range matches {
			ownerRepo, issueN := match[1], match[2]

			// no owner provided-- check config for owner
			if strings.IndexByte(ownerRepo, '/') == -1 {
				var ok bool
				ownerRepo, ok = c.ProjectsByRepoName[strings.ToLower(ownerRepo)]
				if !ok {
					continue
				}
			}

			if len(match) < 3 {
				continue
			}
			u, err := url.Parse(fmt.Sprintf("https://api.github.com/repos/%s/issues/%s", ownerRepo, issueN))
			if err != nil {
				log.Println(err)
				continue
			}
			q := u.Query()
			q.Add("access_token", c.Github.Token)
			u.RawQuery = q.Encode()
			resp, err := http.Get(u.String())
			if err != nil {
				log.Println(err)
				continue
			}
			if !(200 <= resp.StatusCode && resp.StatusCode <= 299) {
				log.Println(resp.Status)
				continue
			}
			defer resp.Body.Close()
			m := make(map[string]interface{})
			if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
				log.Println(err)
				continue
			}
			ircproj.Privmsgf(event.Arguments[0], "[\002#%v\002] %v %v", m["number"].(float64), m["title"].(string), m["html_url"].(string))
		}
	})

	ircproj.Loop()
}
