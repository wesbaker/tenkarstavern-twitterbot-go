package main

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/ChimeraCoder/anaconda"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/joho/godotenv"
	"github.com/mmcdole/gofeed"
	log "github.com/sirupsen/logrus"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// Tweet of URL from feed that has already been sent
type Tweet struct {
	ID        bson.ObjectId `bson:"_id,omitempty"`
	Title     string
	URL       string
	Timestamp time.Time
}

// tweetItem sends out tweet after verifying that it hasn't already been sent
func tweetItem(api *anaconda.TwitterApi, s *mgo.Session, item *gofeed.Item) {
	session := s.Copy()
	defer session.Close()

	tweets := session.DB(os.Getenv("MONGODB_DB")).C("tweets")

	// Check with MongoDB
	count, _ := tweets.Find(bson.M{"url": item.Link}).Count()
	if count > 0 {
		log.WithField("url", item.Link).Debug("Item already exists")
		return
	}

	// Tweet Item
	tweet := fmt.Sprintf("%s\n%s", item.Title, item.Link)
	if os.Getenv("ENVIRONMENT") != "development" {
		_, err := api.PostTweet(tweet, url.Values{})
		if err != nil {
			log.Fatal(err)
		}
	}
	log.WithFields(log.Fields{
		"title": item.Title,
		"url":   item.Link,
		"tweet": tweet,
	}).Debug("Tweeted item")

	// Add to MongoDB
	err := tweets.Insert(&Tweet{Title: item.Title, URL: item.Link, Timestamp: time.Now()})
	if err != nil {
		log.Error(err)
	}
}

// getFeedItems gets the feed items from the RSS feed
func getFeedItems() []*gofeed.Item {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(os.Getenv("RSS_FEED_URL"))

	if err != nil {
		log.Fatal(err)
	}

	return feed.Items
}

// ensureIndex ensures an index was created for the tweets collection
func ensureIndex(s *mgo.Session) {
	session := s.Copy()
	defer session.Close()

	c := session.DB(os.Getenv("MONGODB_DB")).C("tweets")

	index := mgo.Index{
		Key:        []string{"url"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}

	err := c.EnsureIndex(index)
	if err != nil {
		log.Error(err)
	}
}

// tweetFeed sets up the TwitterAPI, connects to MongoDB, ensures the index, and
// then tweets out the feed
func tweetFeed() {
	err := godotenv.Load()
	if err != nil {
		log.Info("No .env file")
	}

	api := anaconda.NewTwitterApiWithCredentials(
		os.Getenv("TWITTER_ACCESS_TOKEN"),
		os.Getenv("TWITTER_ACCESS_TOKEN_SECRET"),
		os.Getenv("TWITTER_CONSUMER_KEY"),
		os.Getenv("TWITTER_CONSUMER_SECRET"),
	)
	_, err = api.VerifyCredentials()
	if err != nil {
		log.Fatal("Could not connect to Twitter")
	}

	session, err := mgo.Dial(os.Getenv("MONGODB_URL"))
	if err != nil {
		log.Fatal(err)
	}

	ensureIndex(session)

	if os.Getenv("ENVIRONMENT") == "development" {
		log.SetLevel(log.DebugLevel)
	}

	for _, item := range getFeedItems() {
		tweetItem(api, session, item)
	}
}

func main() {
	lambda.Start(tweetFeed)
}