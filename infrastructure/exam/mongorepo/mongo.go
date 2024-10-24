package mongorepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Marlliton/go-quizzer/domain/exam"
	"github.com/Marlliton/go-quizzer/domain/fail"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var errMongoCode = "MongoRepository"

type MongoRepository struct {
	db   *mongo.Database
	exam *mongo.Collection
}

type mongoExam struct {
	ID          string           `bson:"_id"`
	Title       string           `bson:"title"`
	Description string           `bson:"description"`
	Questions   []*mongoQuestion `bson:"questions"`
}

type mongoQuestion struct {
	ID        string               `bson:"id"`
	Statement string               `bson:"statement"`
	Items     []*mongoQuestionItem `bson:"items"`
}

type mongoQuestionItem struct {
	ID    string `bson:"id"`
	Text  string `bson:"text"`
	Right bool   `bson:"right"`
}

func (me *mongoExam) toAggregate() (*exam.Exam, error) {
	var questions []*exam.Question
	for _, mq := range me.Questions {
		questionItems, err := me.buildQuestionItems(mq)
		if err != nil {
			return nil, fmt.Errorf("erro creating question items %s, %v", mq.ID, err)
		}
		ques, err := exam.NewQuestion(mq.ID, mq.Statement, questionItems)
		if err != nil {
			return nil, fmt.Errorf("erro creating question %v", err)
		}

		questions = append(questions, ques)
	}

	return exam.NewExam(me.ID, me.Title, me.Description, nil)
}
func (*mongoExam) buildQuestionItems(mq *mongoQuestion) ([]*exam.QuestionItem, error) {
	questionItems := make([]*exam.QuestionItem, len(mq.Items))
	for i, mqi := range mq.Items {
		item, err := exam.NewQuestionItem(mqi.ID, mqi.Text, mqi.Right)
		if err != nil {
			return nil, fmt.Errorf("erro creating item %v", err)
		}
		questionItems[i] = item
	}

	return questionItems, nil
}

func newFromExam(ex exam.Exam) mongoExam {
	mongoQuestions := make([]*mongoQuestion, len(ex.GetQuestions()))

	for i, q := range ex.GetQuestions() {
		mongoItems := make([]*mongoQuestionItem, len(q.GetItems()))

		for j, qi := range q.GetItems() {
			mongoItems[j] = &mongoQuestionItem{
				ID:    qi.GetID(),
				Text:  qi.GetText(),
				Right: qi.GetIsRight(),
			}
		}
		mongoQuestions[i] = &mongoQuestion{
			ID:        q.GetID(),
			Statement: q.GetStatement(),
			Items:     mongoItems,
		}
	}

	return mongoExam{
		ID:          ex.GetID(),
		Description: ex.GetDescription(),
		Title:       ex.GetTitle(),
		Questions:   mongoQuestions,
	}

}

func New(ctx context.Context, uriConnection string) (*MongoRepository, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uriConnection))
	if err != nil {
		return nil, err
	}

	// TODO: Add environment vairable
	db := client.Database("quizzer")
	collectionExam := db.Collection("exam")
	repo := MongoRepository{
		db:   db,
		exam: collectionExam,
	}

	return &repo, nil
}

func (mr *MongoRepository) Get(id string) (*exam.Exam, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result := mr.exam.FindOne(ctx, bson.D{{Key: "_id", Value: id}})

	var me mongoExam

	if err := result.Decode(&me); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fail.WithNotFoundError(errMongoCode, "exam not found")
		}
		return nil, err
	}

	return me.toAggregate()
}

func (mr *MongoRepository) GetAll() ([]*exam.Exam, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := mr.exam.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var exams []*exam.Exam
	for cursor.Next(ctx) {
		var mongoEx mongoExam

		if err := cursor.Decode(&mongoEx); err != nil {
			return nil, fmt.Errorf("erro decoding exam %v", err)
		}

		aggregateExam, err := mongoEx.toAggregate()
		if err != nil {
			return nil, err
		}

		exams = append(exams, aggregateExam)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("erro while iterating over exams %v", err)
	}

	return exams, nil
}

func (mr *MongoRepository) Save(exAdd *exam.Exam) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	doc := newFromExam(*exAdd)

	_, err := mr.exam.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("erro inserting exam into mongoDB %v", err)
	}
	return nil
}

func (mr *MongoRepository) Update(updEx *exam.Exam) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	updatedDoc := newFromExam(*updEx)

	_, err := mr.exam.UpdateOne(ctx, bson.D{{Key: "_id", Value: updatedDoc.ID}}, updatedDoc)
	if err != nil {
		return fmt.Errorf("erro updating exam %s, %v", updatedDoc.ID, err)
	}

	return nil
}

func (mr *MongoRepository) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := mr.exam.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	if err != nil {
		return fmt.Errorf("erro deleting exam %s, %v", id, err)
	}

	return nil
}
