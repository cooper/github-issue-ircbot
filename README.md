# github-issue-ircbot

```github-issue-ircbot``` responds to irc messages that mention github issues with issue information.

```
4:01:17 PM <~mad> #1
4:01:17 PM <issues> [#1] Bug: Error Start https://github.com/cooper/juno/issues/1
4:02:28 PM <~mad> pylink#99
4:02:28 PM <issues> [#99] PyLink S2S protocol - Something that could mimic InterJanus https://github.com/jlu5/PyLink/issues/99
4:02:31 PM <~mad> quiki#31
4:02:31 PM <issues> [#31] wikifier/formatter: invalid links to not-yet-existent categories https://github.com/cooper/quiki/issues/31
```

## how to install

```
go install github.com/cooper/github-issue-ircbot
```

## configuration

JSON file:

```json
{
        "irc": {
                "host": "irc.example.com",
                "port": "6667",
                "ssl": false,
                "ssl_verify_skip": false,
                "channels": [ "#chan", "#another" ],
                "password": "password for irc server",
                "nickname": "issues",
                "realname": "I got issues",
                "ignore": [ "spamuser" ]
        },
        "github": {
                "token": "asdfghjklasdfghjkl",
                "default_owner": "mygithubname",
                "default_repo": "myproject",
                "projects": [
                        "someoneelse/otherproject",
                        "differentperson/anotheryet"
                ]
        }
}
```

## usage

```
github-issue-ircbot --config /path/to/config.json
```

## see also

* http://blog.handlena.me/entry/2013/06/12/234712
* http://soh335.hatenablog.com/entry/2013/06/13/103457

## license

* MIT
