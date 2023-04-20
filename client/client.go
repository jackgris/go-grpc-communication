// Package main implements a simple gRPC client that demonstrates how to use gRPC-Go libraries
// to perform unary, client streaming, server streaming and full duplex RPCs.
//
// It interacts with the person guide service whose definition can be found in personguide/person_guide.proto.
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

// printPhone get the phone from the person with send.
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

// printPersons lists all the persons in same adress.
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

// runRecordPersons sends a sequence of persons to server and expects to get a summary of all persons from server.
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

// runRoutePhones receives a sequence of route phones, while sending a list of persons.
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
			phone, err := stream.Recv()
			if err == io.EOF {
				// read done.
				close(waitc)
				return
			}
			if err != nil {
				log.Fatalf("client.RoutePhones failed: %v", err)
			}
			log.Printf("Got phone %s type %v", phone.Number, phone.Type)
		}
	}()
	for p := range persons {
		if err := stream.Send(&persons[p]); err != nil {
			log.Fatalf("client.RoutePhones: stream.Send(%v) failed: %v", &persons[p], err)
		}
	}
	// For now we don't check errors, don't do this in production
	_ = stream.CloseSend()
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
