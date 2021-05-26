package main

import (
	"context"
	"fmt"
	wordsearcher "github.com/jwjones2/wordsearcher-server/wspb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"os"
	"os/signal"
	"time"
)

// define global mongo variables
var db *mongo.Client
var collection *mongo.Collection
var mongoCtx context.Context

// empty struct for the gRpc server
type server struct {
	wordsearcher.UnimplementedWordsearcherServiceServer
}

/* TODO - User
 */

// Verse struct for the Database items to return
type Verse struct {
	ID       primitive.ObjectID `bson:"id"`
	Book     int32              `bson:"book"`
	BookName string             `bson:"book_name"`
	Chapter  int32              `bson:"chapter"`
	Verse    int32              `bson:"verse"`
	Text     string             `bson:"text"`
	Keywords string             `bson:"keywords"`
}

// BiblePlan Bible plan struct to return Bible Plans
type BiblePlan struct {
	ID     primitive.ObjectID `bson:"id"`
	Name   string             `bson:"name"`
	Number int32              `bson:"number"`
	Days   []string           `bson:"days"`
}

// BiblePlanDay Bible plan day struct for specific day of the Bible Plan
type BiblePlanDay struct {
	ID      primitive.ObjectID `bson:"id"`
	Reading string             `bson:"reading"`
}

// CustomRange Custom Range table
type CustomRange struct {
	ID          primitive.ObjectID `bson:"id"`
	Name        string             `bson:"name"`
	Type        string             `bson:"type"`
	BookNumber  int32              `bson:"booknumber"`
	CustomRange []int32            `bson:"customrange"`
}

func (s server) Verse(ctx context.Context, request *wordsearcher.VerseRequest) (*wordsearcher.VerseResponse, error) {
	// Functionality: Query the verse table for verse based on range
	// 	- If the verse_start == 0 return the whole chapter.
	// 	- verse_start and verse_end will determine a range or single verse (both equal).
	//
	// ** Error handling:
	// 		- if verse_start < 1 return out of range error
	// 		- if verse_start > verse_end return out of range error

	// first get the proper collection
	collection = db.Database("myFirstDatabase").Collection("verse")

	// check the verse_start and verse_end variables, return errors if necessary
	// then build the filter for the search.
	if request.VerseStart < 0 {
		return nil, status.Errorf(codes.OutOfRange, "The verse range must be positive. Invalid: %v", request.VerseStart)
	}
	if request.VerseStart > request.VerseEnd {
		return nil, status.Errorf(codes.OutOfRange,
			"The start of the verse range cannot be greater than the end. Invalid: Verse Start: %v; Verse End: %v",
			request.VerseStart, request.VerseEnd)
	}

	// build the filter
	var filter bson.M
	if request.VerseStart == 0 {
		// retrieve the whole chapter instead of a range of verses
		filter = bson.M{
			"book":    request.GetBook(),
			"chapter": request.GetChapter(),
		}
	} else {
		// select a range of verses
		filter = bson.M{
			"book":    request.GetBook(),
			"chapter": request.GetChapter(),
			"verse": bson.M{
				"$gte": request.VerseStart,
				"$lte": request.VerseEnd,
			},
		}
	}

	// do the Database call and store the results
	var verses []*Verse
	verseCursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("Error finding the verses: %v", err.Error()))
	}
	if cursorErr := verseCursor.All(ctx, &verses); cursorErr != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Error decoding the cursor into verses: %v", cursorErr.Error()))
	}

	// build the protocol buffer response
	var verseResponses []*wordsearcher.Verse
	for _, verse := range verses {
		verseResponses = append(verseResponses, &wordsearcher.Verse{
			Book:     verse.Book,
			Chapter:  verse.Chapter,
			BookName: verse.BookName,
			Verse:    verse.Verse,
			Text:     verse.Text,
			Keywords: verse.Keywords,
		})
	}

	// return the results to the client
	return &wordsearcher.VerseResponse{
		Verses: verseResponses,
	}, nil
}

func (s server) BiblePlan(ctx context.Context, request *wordsearcher.BiblePlanRequest) (*wordsearcher.BiblePlanResponse, error) {
	// Functionality:
	// - Finds a bible plan by name and returns.

	// first get the proper collection
	collection = db.Database("myFirstDatabase").Collection("readingplan")

	// build filter and search
	filter := bson.M{
		"name": request.GetName(),
	}
	var biblePlans []BiblePlan
	planCursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("Error finding the bible plan: %v", err.Error()))
	}
	if cursorErr := planCursor.All(mongoCtx, &biblePlans); cursorErr != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Error decoding the cursor into plans: %v", cursorErr.Error()))
	}

	// build the protocol buffer response
	var biblePlanResponse []*wordsearcher.BiblePlan
	for _, biblePlan := range biblePlans {
		biblePlanResponse = append(biblePlanResponse, &wordsearcher.BiblePlan{
			Name:   biblePlan.Name,
			Number: biblePlan.Number,
			Days:   biblePlan.Days,
		})
	}

	return &wordsearcher.BiblePlanResponse{
		BiblePlan: biblePlanResponse,
	}, nil
}

func (s server) BiblePlanDay(ctx context.Context, request *wordsearcher.BiblePlanDayRequest) (*wordsearcher.BiblePlanDayResponse, error) {
	// Functionality
	// - Request a specific day in the Bible plan and return
	//
	// **Error Handling
	// - If day is less than 0 return out of range error

	// first get the proper collection
	collection = db.Database("myFirstDatabase").Collection("readingplan")

	// check the verse_start and verse_end variables, return errors if necessary
	// then build the filter for the search.
	if request.Day < 0 {
		return nil, status.Errorf(codes.OutOfRange, "The Bible Plan Day must be positive. Invalid: %v", request.Day)
	}

	// Search the Database and return
	// need to use an aggregate to be able to return the proper format
	t := time.Now()
	cDay := t.YearDay() - 1
	matchStage := bson.D{
		{"$match", bson.D{
			{"name", "McCheyneBasedYearly"},
		}},
	}
	projectStage := bson.D{
		{"$project", bson.M{
			"reading": bson.M{
				"$arrayElemAt": bson.A{
					"$days", cDay,
				},
			},
		}},
	}
	planCursor, err := collection.Aggregate(ctx, mongo.Pipeline{matchStage, projectStage})
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Could not find the bible plan day: %v", err)
	}

	// unpack the found plan days
	var biblePlanDays []BiblePlanDay
	if cursorErr := planCursor.All(ctx, &biblePlanDays); cursorErr != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Error decoding the cursor into plans: %v", cursorErr.Error()))
	}

	// build the protocol buffer response
	biblePlanDayResponse := &wordsearcher.BiblePlanDay{
		Name:     request.GetName(),
		Reading1: biblePlanDays[0].Reading,
		Reading2: biblePlanDays[1].Reading,
		Reading3: biblePlanDays[2].Reading,
		Reading4: biblePlanDays[3].Reading,
	}

	return &wordsearcher.BiblePlanDayResponse{
		Day: biblePlanDayResponse,
	}, nil
}

func (s server) Search(ctx context.Context, request *wordsearcher.SearchRequest) (*wordsearcher.VerseResponse, error) {
	// Functionality
	// - Takes a search term and filters (filter [type of search, any term, exact term...], location [in Scriptures],
	//   options [extra optional values to specify when more information needed from filter and location]
	// - Builds the filters for searching
	// - Searches and returns matching verses
	//
	// * Defaults to search everywhere, match any terms

	// first get the proper collection
	collection = db.Database("myFirstDatabase").Collection("verse")

	// build the stages for the mongo Pipeline, project and sort remain the same for any kind of search
	projectStage := bson.D{
		{"$project", bson.D{
			{"book", 1},
			{"bookname", 1},
			{"chapter", 1},
			{"verse", 1},
			{"text", 1},
			{"score", bson.M{
				"$meta": "searchScore",
			}},
		}},
	}
	sortStage := bson.D{
		{"$sort", bson.D{
			{"score", -1},
		}},
	}
	var filterStage bson.D

	// first check if the default case, no filter or location, is true
	// if so, create the default search filter, else test conditions and
	// build the filter from filter, location, and options
	// set search filter and search location to blank to be able to check both values
	// since if they are both blank, the default case is true, and potentially all could
	// be passed instead of just blank
	searchFilter := request.GetFilter()
	if searchFilter == "all" {
		searchFilter = ""
	}
	searchLocation := request.GetLocation()
	if searchLocation == "all" {
		searchLocation = ""
	}
	if searchFilter == "" && searchLocation == "" {
		filterStage = bson.D{
			{"$search", bson.D{
				{"text", bson.D{
					{"path", "text"},
					{"query", request.GetTerm()},
				}},
			}},
		}
	} else {
		var matchDoc bson.D
		var locationDoc bson.D

		// Build the query stages for the Pipeline
		// test for in or exact - any terms, or exact phrase match
		switch request.GetFilter() {
		case "exact":
			matchDoc = bson.D{
				{"phrase", bson.D{
					{"path", "text"},
					{"query", request.GetTerm()},
					/* TODO - May need to add distance in the future "slop": <distance of words apart - 0 is default, means words right next to each other */
				}}}
		case "in": // ??? TODO - this is the same as default???
		default: // all or blank, search to match any terms (Mongo default as well)
			matchDoc = bson.D{
				{"text", bson.D{
					{"path", "text"},
					{"query", request.GetTerm()},
				}}}
		}

		switch request.GetLocation() {
		case "nt": // only the New Testament, books > 39
			locationDoc = bson.D{
				{"range", bson.D{
					{"path", "book"},
					{"gt", 39},
				}}}

		case "ot": // only search in the Old Testament, book <= 39
			locationDoc = bson.D{
				{"range", bson.D{
					{"path", "book"},
					{"lte", 39},
				}}}
		case "law": // first 5 books of the Bible, Penteteuch
			locationDoc = bson.D{
				{"range", bson.D{
					{"path", "book"},
					{"lte", 5},
				}}}
		case "bookname": // allows a book name to be passed on Options
			/* TODO - Need to finish this, set to match a bookname passed on GetOptions */
			locationDoc = bson.D{
				{"range", bson.D{
					{"path", "book"},
					{"gt", 39},
				}}}
			/* TODO - Add other searchable locations, like poetry... */
		default: // search any matching terms
			locationDoc = bson.D{
				{"range", bson.D{
					{"path", "book"},
					{"gt", 0},
				}}}
		}

		// build the filterStage from the above matchDoc and locationDoc
		filterStage = bson.D{
			{"$search", bson.D{
				{"compound", bson.D{
					{"must", bson.A{
						matchDoc,
						locationDoc,
					}},
				}},
			}},
		}

		fmt.Println(filterStage)
	}

	verseCursor, err := collection.Aggregate(ctx, mongo.Pipeline{filterStage, projectStage, sortStage})
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Could not complete the verse search: %v", err)
	}

	// unpack the verses from the cursor
	var verses []*Verse
	if cursorErr := verseCursor.All(mongoCtx, &verses); cursorErr != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Error decoding the cursor into verses: %v", cursorErr.Error()))
	}

	// build the protocol buffer response
	var verseResponses []*wordsearcher.Verse
	for _, verse := range verses {
		verseResponses = append(verseResponses, &wordsearcher.Verse{
			Book:     verse.Book,
			Chapter:  verse.Chapter,
			BookName: verse.BookName,
			Verse:    verse.Verse,
			Text:     verse.Text,
			Keywords: verse.Keywords,
		})
	}

	// return the results to the client
	return &wordsearcher.VerseResponse{
		Verses: verseResponses,
	}, nil
}

func (s server) BookRange(ctx context.Context, request *wordsearcher.BookRangeRequest) (*wordsearcher.VerseResponse, error) {
	// Functionality:
	// - Query verses based on book start and book end inclusive.
	//
	// **Error handling
	// - If book start negative, return out of bounds error
	// - If book start greater than book end, return out of bounds error
	// - If book start and book end equal, returns one book

	// first get the proper collection
	collection = db.Database("myFirstDatabase").Collection("verse")

	// validation - error handling
	if request.GetStart() < 0 {
		return nil, status.Errorf(codes.OutOfRange, "The book range must be positive. Invalid: %v", request.GetStart())
	}
	if request.GetStart() > request.GetEnd() {
		return nil, status.Errorf(codes.OutOfRange,
			"The start of the book range cannot be greater than the end. Invalid: Book Start: %v; Book End: %v",
			request.GetStart(), request.GetEnd())
	}

	// build the filter
	filter := bson.M{
		"book": bson.M{
			"$gte": request.GetStart(),
			"$lte": request.GetEnd(),
		},
	}

	// do the Database call and store the results
	var verses []*Verse
	verseCursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("Error finding the verses: %v", err.Error()))
	}
	if cursorErr := verseCursor.All(mongoCtx, &verses); cursorErr != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Error decoding the cursor into verses: %v", cursorErr.Error()))
	}

	// build the protocol buffer response
	var verseResponses []*wordsearcher.Verse
	for _, verse := range verses {
		verseResponses = append(verseResponses, &wordsearcher.Verse{
			Book:     verse.Book,
			Chapter:  verse.Chapter,
			BookName: verse.BookName,
			Verse:    verse.Verse,
			Text:     verse.Text,
			Keywords: verse.Keywords,
		})
	}

	// return the results to the client
	return &wordsearcher.VerseResponse{
		Verses: verseResponses,
	}, nil
}

func (s server) ChapterRange(ctx context.Context, request *wordsearcher.ChapterRangeRequest) (*wordsearcher.VerseResponse, error) {
	// Functionality:
	// - Query verses based on chapters start and end in a particular book - a range of chapters in Genesis, for instance.
	//
	// **Error handling
	// - If book is negative return out of bounds.
	// - If chatper start is negative return out of bounds.
	// - If chapter start is larger than chapter end return out of bounds.

	// first get the proper collection
	collection = db.Database("myFirstDatabase").Collection("verse")

	// validation - error handling
	if request.GetBook() < 0 {
		return nil, status.Errorf(codes.OutOfRange, "The book must be positive. Invalid: %v", request.GetBook())
	}
	if request.GetStart() < 0 {
		return nil, status.Errorf(codes.OutOfRange, "The chapter range must be positive. Invalid: %v", request.GetStart())
	}
	if request.GetStart() > request.GetEnd() {
		return nil, status.Errorf(codes.OutOfRange,
			"The start of the chapter range cannot be greater than the end. Invalid: Chapter Start: %v; Chapter End: %v",
			request.GetStart(), request.GetEnd())
	}

	// build the filter
	filter := bson.M{
		"book": request.GetBook(),
		"chapter": bson.M{
			"$gte": request.GetStart(),
			"$lte": request.GetEnd(),
		},
	}

	// do the Database call and store the results
	var verses []*Verse
	verseCursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("Error finding the verses: %v", err.Error()))
	}
	if cursorErr := verseCursor.All(mongoCtx, &verses); cursorErr != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Error decoding the cursor into verses: %v", cursorErr.Error()))
	}

	// build the protocol buffer response
	var verseResponses []*wordsearcher.Verse
	for _, verse := range verses {
		verseResponses = append(verseResponses, &wordsearcher.Verse{
			Book:     verse.Book,
			Chapter:  verse.Chapter,
			BookName: verse.BookName,
			Verse:    verse.Verse,
			Text:     verse.Text,
			Keywords: verse.Keywords,
		})
	}

	// return the results to the client
	return &wordsearcher.VerseResponse{
		Verses: verseResponses,
	}, nil
}

func (s server) CustomRange(ctx context.Context, request *wordsearcher.CustomRangeRequest) (*wordsearcher.CustomRangeResponse, error) {
	// Functionality
	// - Query the Custom Range table and return the results

	// first get the proper collection
	collection = db.Database("myFirstDatabase").Collection("customrange")

	// build filter and query
	filter := bson.D{
		{"name", request.GetName()},
	}
	res := collection.FindOne(ctx, filter)
	if res.Err() != nil {
		return nil, status.Errorf(codes.NotFound, "Could not find the custom range named %s; %v.", request.GetName(), res.Err())
	}

	// marshal the result and build the return response and return
	var cRange *CustomRange
	err := res.Decode(&cRange)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error decoding the custom range...please try again; %v", err)
	}

	// build the response and return
	return &wordsearcher.CustomRangeResponse{
		CustomRange: &wordsearcher.CustomRange{
			Name:        cRange.Name,
			Type:        cRange.Type,
			Booknumber:  cRange.BookNumber,
			Customrange: cRange.CustomRange,
		},
	}, nil
}

func main() {
	// set the logging to be able to catch file name and line number in the log
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// set the logging to be able to catch file name and line number in the log
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// connect to Mongodb and initialize the global variables
	fmt.Println("Connecting to MongoDB...")
	mongoCtx = context.Background()
	mongoURI := os.Getenv("MONGOURI")
	var err error
	db, err = mongo.Connect(mongoCtx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("Error connecting to MongoDB client: %v", err)
	}
	defer func() {
		if err = db.Disconnect(mongoCtx); err != nil {
			panic(err)
		}
	}()

	// Start the server
	fmt.Println("WordSearcher Server started!")
	lis, err := net.Listen("tcp", "0.0.0.0:50051")
	if err != nil {
		log.Fatalf("Failed to run the server: %v", err)
	}

	s := grpc.NewServer()
	wordsearcher.RegisterWordsearcherServiceServer(s, &server{})

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Use a Wait channel to gracefully stop the servewr
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	// block the rest of the code until interrupt signal
	<-ch

	// Now Properly stop the server
	fmt.Println("Stopping the server...")
	s.Stop()
	fmt.Println("Closing the listener...")
	err = lis.Close()
	if err != nil {
		return
	}
	fmt.Println("The server is stopped!")
}
