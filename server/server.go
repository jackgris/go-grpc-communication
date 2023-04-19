// Package main implements a simple gRPC server that demonstrates how to use gRPC-Go libraries
// to perform unary, client streaming, server streaming and full duplex RPCs.
//
// It implements the route guide service whose definition can be found in routeguide/route_guide.proto.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"google.golang.org/grpc/credentials"

	"github.com/jackgris/go-grpc-communication/data"
	pb "github.com/jackgris/go-grpc-communication/personguide"
)

var (
	tls        = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	certFile   = flag.String("cert_file", "", "The TLS cert file")
	keyFile    = flag.String("key_file", "", "The TLS key file")
	jsonDBFile = flag.String("json_db_file", "", "A json file containing a list of features")
	port       = flag.Int("port", 50051, "The server port")
)

type routeGuideServer struct {
	pb.UnimplementedPersonGuideServer
	savedPersons []*pb.Person // read-only after initialized

	mu          sync.Mutex // protects routeNotes
	routeAdress map[string][]*pb.AddressBook
}

// GetFeature returns the feature at the given point.
func (s *routeGuideServer) GetPhone(ctx context.Context, person *pb.Person) (*pb.PhoneNumber, error) {
	for _, p := range s.savedPersons {
		if p.Id == person.Id {
			return p.GetPhones()[0], nil
		}
	}
	// No feature was found, return an unnamed feature
	return &pb.PhoneNumber{}, errors.New("Not found person")
}

// ListFeatures lists all features contained within the given bounding Rectangle.
func (s *routeGuideServer) ListPersons(adress *pb.Adress, stream pb.PersonGuide_ListPersonsServer) error {
	fmt.Println("In list persons with adress: ", adress)
	for _, person := range s.savedPersons {
		if err := stream.Send(person); err != nil {
			return err
		}
	}
	return nil
}

// RecordRoute records a route composited of a sequence of points.
//
// It gets a stream of points, and responds with statistics about the "trip":
// number of points,  number of known features visited, total distance traveled, and
// total time spent.
func (s *routeGuideServer) RecordPersons(stream pb.PersonGuide_RecordPersonsServer) error {
	var lastPerson *pb.Person
	for {
		person, err := stream.Recv()
		if err != nil && person != nil {
			ts := timestamppb.New(time.Now())
			lastPerson = person
			lastPerson.LastUpdated = ts
			s.savedPersons = append(s.savedPersons, lastPerson)
		}
		if err == io.EOF {
			return stream.SendAndClose(&pb.AddressBook{
				People: s.savedPersons,
			})
		}
		if err != nil {
			return err
		}
	}
}

// RouteChat receives a stream of message/location pairs, and responds with a stream of all
// previous messages at each of those locations.
func (s *routeGuideServer) RoutePhones(stream pb.PersonGuide_RoutePhonesServer) error {
	for {
		person, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		s.mu.Lock()
		// Note: this copy prevents blocking other clients while serving this one.
		// We don't need to do a deep copy, because elements in the slice are
		// insert-only and never modified.
		rn := make([]*pb.PhoneNumber, len(person.Phones))
		copy(rn, person.Phones)
		s.mu.Unlock()

		for _, phone := range rn {
			if err := stream.Send(phone); err != nil {
				return err
			}
		}
	}
}

// loadFeatures loads features from a JSON file or database.
func (s *routeGuideServer) loadFeatures(filePath string) {
	fmt.Println("In future we can load data from the filepath: ", filePath)
	s.savedPersons = exampleData
}

func newServer() *routeGuideServer {
	s := &routeGuideServer{routeAdress: make(map[string][]*pb.AddressBook)}
	s.loadFeatures(*jsonDBFile)
	return s
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	if *tls {
		if *certFile == "" {
			*certFile = data.Path("x509/server_cert.pem")
		}
		if *keyFile == "" {
			*keyFile = data.Path("x509/server_key.pem")
		}
		creds, err := credentials.NewServerTLSFromFile(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("Failed to generate credentials: %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterPersonGuideServer(grpcServer, newServer())
	err = grpcServer.Serve(lis)
	if err != nil {
		log.Fatalf("Fail while server running: %v", err)
	}
}

// Example data
var phones = []*pb.PhoneNumber{
	{Number: "1234", Type: pb.PhoneType_HOME},
	{Number: "4321", Type: pb.PhoneType_WORK},
	{Number: "4312", Type: pb.PhoneType_MOBILE},
}

var exampleData = []*pb.Person{
	{Name: "Juan", Id: 1, Email: "juan@gmail.com", Phones: phones},
	{Name: "Gabriel", Id: 2, Email: "gabriel@gmail.com", Phones: phones},
	{Name: "Albert", Id: 3, Email: "albert@gmail.com", Phones: phones},
	{Name: "Mark", Id: 4, Email: "mark@gmail.com", Phones: phones},
	{Name: "Brian", Id: 5, Email: "brian@gmail.com", Phones: phones},
	{Name: "Kevin", Id: 6, Email: "kevin@gmail.com", Phones: phones},
}
