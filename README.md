# Glipbot Tokenizer

[![Build Status][build-status-svg]][build-status-url]
[![Lint Status][lint-status-svg]][lint-status-url]
[![Go Report Card][goreport-svg]][goreport-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![License][license-svg]][license-url]
[![Heroku][heroku-svg]][heroku-url]
[![Video][video-svg]][video-url]
[![Stack Overflow][stackoverflow-svg]][stackoverflow-url]
[![Chat][chat-svg]][chat-url]

Helper app to retrieve Glip bot access token.

This app allows you to retrieve a token without coding OAuth into your app.

* Online Service: [https://glipbot-tokenizer.herokuapp.com](https://glipbot-tokenizer.herokuapp.com)
* YouTube Tutorial Video: [https://youtu.be/A7T7xDGV5vU](https://youtu.be/A7T7xDGV5vU)

Note: this works for private bots only.

## Installation

Note: if you just want to retrieve a token for your bot, you can simply use the [online service](https://glipbot-tokenizer.herokuapp.com). The below is if you want to host your own version of Glipbot Tokenizer.

### Deploying to Heroku

```sh
$ heroku create
$ git push heroku master
$ heroku open
```

or

[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy)

More information on deploying Go on Heroku here:

* https://devcenter.heroku.com/articles/go-support

## Configuration

| Environment Variable | Required | Description |
|----------------------|----------|-------------|
| `APP_SERVER_URL`     | y | Base URL for your server, e.g. `https://myapp.herokuapp.com` |
| `SPARKPOST_API_KEY`  | y | Your SparkPost API Key (https://sparkpost.com) to send email |
| `SPARKPOST_EMAIL_SENDER` | y | Your sender email address. The domain must be verified by SparkPost |
| `HTTP_ENGINE` | n | HTTP engine. Currently `nethttp` is supported |

 [build-status-svg]: https://github.com/grokify/glipbot-tokenizer/workflows/test/badge.svg
 [build-status-url]: https://github.com/grokify/glipbot-tokenizer/actions/workflows/test.yaml
 [lint-status-svg]: https://github.com/grokify/glipbot-tokenizer/workflows/lint/badge.svg
 [lint-status-url]: https://github.com/grokify/glipbot-tokenizer/actions/workflows/lint.yaml
 [goreport-svg]: https://goreportcard.com/badge/github.com/grokify/glipbot-tokenizer
 [goreport-url]: https://goreportcard.com/report/github.com/grokify/glipbot-tokenizer
 [docs-godoc-svg]: https://img.shields.io/badge/docs-godoc-blue.svg
 [docs-godoc-url]: https://godoc.org/github.com/grokify/glipbot-tokenizer
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/grokify/glipbot-tokenizer/blob/master/LICENSE.md
 [heroku-svg]: https://img.shields.io/badge/%E2%86%91_deploy-Heroku-7056bf.svg?style=flat
 [heroku-url]: https://heroku.com/deploy
 [video-svg]: https://img.shields.io/badge/YouTube-tutorial-red.svg
 [video-url]: https://youtu.be/A7T7xDGV5vU
 [chat-svg]: https://img.shields.io/badge/%F0%9F%92%AC_Chat_on-Glip-orange.svg?style=flat
 [chat-url]: https://glipped.herokuapp.com/
 [stackoverflow-svg]: https://img.shields.io/badge/stack%20overflow-ringcentral-orange.svg
 [stackoverflow-url]: https://stackoverflow.com/questions/tagged/ringcentral
