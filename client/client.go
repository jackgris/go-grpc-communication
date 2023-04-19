// Package main implements a simple gRPC client that demonstrates how to use gRPC-Go libraries
// to perform unary, client streaming, server streaming and full duplex RPCs.
//
// It interacts with the route guide service whose definition can be found in routeguide/route_guide.proto.
package main

import (
	"context"
	"flag"
	"io"
	"log"
	"time"

	"github.com/jackgris/go-grpc-communication/data"
	pb "github.com/jackgris/go-grpc-communication/personguide"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	tls                = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	caFile             = flag.String("ca_file", "", "The file containing the CA root cert file")
	serverAddr         = flag.String("addr", "localhost:50051", "The server address in the format of host:port")
	serverHostOverride = flag.String("server_host_override", "x.test.example.com", "The server name used to verify the hostname returned by the TLS handshake")
)

// printFeature gets the feature for the given point.
func printPhone(client pb.PersonGuideClient, person *pb.Person) {
	log.Printf("Getting phone from person %s", person.GetName())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	phone, err := client.GetPhone(ctx, person)
	if err != nil {
		log.Fatalf("client.GetPhone failed: %v", err)
	}
	log.Println(phone)
}

// printFeatures lists all the features within the given bounding Rectangle.
func printPersons(client pb.PersonGuideClient, adress *pb.Adress) {
	log.Printf("Looking for persons in adress %s", adress.GetName())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := client.ListPersons(ctx, adress)
	if err != nil {
		log.Fatalf("client.ListPersons failed: %v", err)
	}
	for {
		person, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("client.ListPersons failed: %v", err)
		}
		log.Printf("Person: name: %s, email:%s, Id: %d\n", person.GetName(),
			person.GetEmail(), person.GetId())
	}
}

// runRecordRoute sends a sequence of points to server and expects to get a RouteSummary from server.
func runRecordPersons(client pb.PersonGuideClient) {
	log.Printf("Traversing %d persons.", len(persons))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := client.RecordPersons(ctx)
	if err != nil {
		log.Fatalf("client.RecordPersons failed: %v", err)
	}
	for p := range persons {
		if err := stream.Send(&persons[p]); err != nil {
			log.Fatalf("client.RecordPersons: stream.Send(%v) failed: %v", &persons[p], err)
		}
	}
	reply, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("client.RecordPersons failed: %v", err)
	}
	log.Printf("AdressBook summary: %v", reply)
}

// runRouteChat receives a sequence of route notes, while sending notes for various locations.
func runRoutePhones(client pb.PersonGuideClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := client.RoutePhones(ctx)
	if err != nil {
		log.Fatalf("client.RoutePhones failed: %v", err)
	}
	waitc := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				close(waitc)
				return
			}
			if err != nil {
				log.Fatalf("client.RoutePhones failed: %v", err)
			}
			log.Printf("Got phone %s type %v", in.Number, in.Type)
		}
	}()
	for p := range persons {
		if err := stream.Send(&persons[p]); err != nil {
			log.Fatalf("client.RoutePhones: stream.Send(%v) failed: %v", &persons[p], err)
		}
	}
	stream.CloseSend()
	<-waitc
}

func main() {
	flag.Parse()
	var opts []grpc.DialOption
	if *tls {
		if *caFile == "" {
			*caFile = data.Path("x509/ca_cert.pem")
		}
		creds, err := credentials.NewClientTLSFromFile(*caFile, *serverHostOverride)
		if err != nil {
			log.Fatalf("Failed to create TLS credentials: %v", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewPersonGuideClient(conn)
	// https://protobuf.dev/getting-started/gotutorial/

	runRecordPersons(client)

	runRoutePhones(client)

	for p := range persons {
		printPhone(client, &persons[p])
	}

	adress := pb.Adress{Name: "my adress"}
	printPersons(client, &adress)
}

// Example data
var phones = []*pb.PhoneNumber{
	{Number: "1234", Type: pb.PhoneType_HOME},
	{Number: "4321", Type: pb.PhoneType_WORK},
	{Number: "4312", Type: pb.PhoneType_MOBILE},
}

var persons = []pb.Person{
	{Name: "Juan", Id: 1, Email: "juan@gmail.com", Phones: phones},
	{Name: "Gabriel", Id: 2, Email: "gabriel@gmail.com", Phones: phones},
	{Name: "Albert", Id: 3, Email: "albert@gmail.com", Phones: phones},
	{Name: "Mark", Id: 4, Email: "mark@gmail.com", Phones: phones},
	{Name: "Brian", Id: 5, Email: "brian@gmail.com", Phones: phones},
	{Name: "Kevin", Id: 6, Email: "kevin@gmail.com", Phones: phones},
}
