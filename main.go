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
	"os"
	"regexp"
	"strings"

	irc "github.com/thoj/go-ircevent"
)

var conf = flag.String("config", "", "configuration file")

type config struct {

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
		Token        string   `json:"token"`
		Projects     []string `json:"projects"`
		DefaultOwner string   `json:"default_owner"`
		DefaultRepo  string   `json:"default_repo"`
	} `json:"github"`

	// internal/caching stuff
	ProjectsByRepoName map[string]string
}

func (c *config) load(filename string) error {
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

	// config file option missing
	if *conf == "" {
		fmt.Fprintln(os.Stderr, "Usage:", os.Args[0], "-config /path/to/config")
		os.Exit(0)
	}

	// load config
	c := &config{ProjectsByRepoName: make(map[string]string)}
	if err := c.load(*conf); err != nil {
		log.Fatal(err)
	}

	// create IRC bot
	ircproj := irc.IRC(c.Irc.Nickname, c.Irc.Nickname)
	ircproj.UseTLS = c.Irc.Ssl
	if c.Irc.SslVerifySkip {
		ircproj.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}
	ircproj.Password = c.Irc.Password

	// connect to IRC
	err := ircproj.Connect(net.JoinHostPort(c.Irc.Host, c.Irc.Port))
	if err != nil {
		log.Fatal(err)
	}

	// autojoin
	ircproj.AddCallback("001", func(event *irc.Event) {
		for _, channel := range c.Irc.Channels {
			ircproj.Join(channel)
		}
	})

	// check messages for issues mentioned
	r := regexp.MustCompile(`([\w/]+)?#(\d+)`)
	ircproj.AddCallback("PRIVMSG", func(event *irc.Event) {

		// check if user is ignored
		for _, ignoreNick := range c.Irc.Ignore {
			if event.Nick == ignoreNick || event.User == ignoreNick {
				return
			}
		}

		// search for every mention of an issue
		matches := r.FindAllStringSubmatch(event.Message(), 1)
		for _, match := range matches {

			// extract parts
			if len(match) < 3 {
				continue
			}
			ownerRepo, issueN := match[1], match[2]

			// only a number provided; try defaultrepo
			if ownerRepo == "" {
				if c.Github.DefaultRepo == "" {
					continue
				}
				ownerRepo = c.Github.DefaultRepo
			}

			// no owner provided-- check config for owner
			if strings.IndexByte(ownerRepo, '/') == -1 {
				found, ok := c.ProjectsByRepoName[strings.ToLower(ownerRepo)]
				if ok {
					ownerRepo = found
				} else {
					if c.Github.DefaultOwner == "" {
						continue
					}
					ownerRepo = c.Github.DefaultOwner + "/" + ownerRepo
				}
			}

			// validate URL
			u, err := url.Parse(fmt.Sprintf("https://api.github.com/repos/%s/issues/%s", ownerRepo, issueN))
			if err != nil {
				log.Println(err)
				continue
			}

			// create request with Authentication header
			req, err := http.NewRequest(http.MethodGet, u.String(), nil)
			if err != nil {
				log.Println(err)
			}
			req.SetBasicAuth("token", c.Github.Token)

			// commit request
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Println(err)
				continue
			}

			// HTTP error occurred
			if !(200 <= resp.StatusCode && resp.StatusCode <= 299) {
				log.Println(resp.Status, u)
				continue
			}

			// decode response
			defer resp.Body.Close()
			m := make(map[string]interface{})
			if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
				log.Println(err)
				continue
			}

			// reply
			ircproj.Privmsgf(event.Arguments[0], "[\002#%v\002] %v %v", m["number"].(float64), m["title"].(string), m["html_url"].(string))
		}
	})

	ircproj.Loop()
}
