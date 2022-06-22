# qlearning

The qlearning package provides a series of interfaces and utilities to implement
the [Q-Learning](https://en.wikipedia.org/wiki/Q-learning) algorithm in
Go.

This project was largely inspired by [flappybird-qlearning-
bot](https://github.com/chncyhn/flappybird-qlearning-bot).

*Until a release is tagged, qlearning should be considered highly
experimental and mostly a fun toy.*

## Installation

```shell
$ go get https://github.com/ecooper/qlearning
```

## Quickstart

qlearning provides example implementations in the [examples](examples/)
directory of the project.

[hangman.go](examples/hangman.go) provides a naive implementation of
[Hangman](https://en.wikipedia.org/wiki/Hangman_(game)) for use with
qlearning.

```shell
$ cd $GOPATH/src/github.com/ecooper/qlearning/examples
$ go run hangman.go -h
Usage of hangman:
  -debug
        Set debug
  -games int
        Play N games (default 5000000)
  -progress int
        Print progress messages every N games (default 1000)
  -wordlist string
        Path to a wordlist (default "./wordlist.txt")
  -words int
        Use N words from wordlist (default 10000)
```

By default, running [hangman.go](examples/hangman.go) will play millions
of games against a 10,000-word corpus. That's a bit overkill for just
trying out qlearning. You can run it against a smaller number of words
for a few number of games using the `-games` and `-words` flags.

```shell
$ go run hangman.go -words 100 -progress 1000 -games 5000
100 words loaded
1000 games played: 92 WINS 908 LOSSES 9% WIN RATE
2000 games played: 447 WINS 1553 LOSSES 36% WIN RATE
3000 games played: 1064 WINS 1936 LOSSES 62% WIN RATE
4000 games played: 1913 WINS 2087 LOSSES 85% WIN RATE
5000 games played: 2845 WINS 2155 LOSSES 93% WIN RATE

Agent performance: 5000 games played, 2845 WINS 2155 LOSSES 57% WIN RATE
```

"WIN RATE" per progress report is isolated within that cycle, a group of
1000 games in this example. The win rate is meant to show the velocity
of learning by the agent. If it is "learning", the win rate should be
increasing until reaching convergence.

As you can see, after 5000 games, the agent is able to "learn" and play
hangman against a 100-word vocabulary.

## Usage

See [godocs](https://godoc.org/github.com/ecooper/qlearning) for the
package documentation.
