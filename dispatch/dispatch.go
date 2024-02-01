package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	pubsub "cloud.google.com/go/pubsub"
)

type MRequest struct {
	Datasets    []string `json:"datasets"`
	Window      [4]int   `json:"window"`
	Destination string   `json:"destination"`
}

var inputs = []string{
	"T31TCJ_20230102T104441_TCI.tif",
	"T31TCJ_20230103T110349_TCI.tif",
	"T31TCJ_20230105T105431_TCI.tif",
	"T31TCJ_20230107T104329_TCI.tif",
	"T31TCJ_20230108T110431_TCI.tif",
	"T31TCJ_20230110T105329_TCI.tif",
	"T31TCJ_20230112T104411_TCI.tif",
	"T31TCJ_20230113T110319_TCI.tif",
	"T31TCJ_20230115T105411_TCI.tif",
	"T31TCJ_20230117T104259_TCI.tif",
	"T31TCJ_20230118T110401_TCI.tif",
	"T31TCJ_20230120T105249_TCI.tif",
	"T31TCJ_20230122T104341_TCI.tif",
	"T31TCJ_20230123T110249_TCI.tif",
	"T31TCJ_20230125T105331_TCI.tif",
	"T31TCJ_20230127T104219_TCI.tif",
	"T31TCJ_20230128T110321_TCI.tif",
	"T31TCJ_20230130T105209_TCI.tif",
	"T31TCJ_20230201T104251_TCI.tif",
	"T31TCJ_20230202T110159_TCI.tif",
	"T31TCJ_20230204T105241_TCI.tif",
	"T31TCJ_20230206T104129_TCI.tif",
	"T31TCJ_20230207T110221_TCI.tif",
	"T31TCJ_20230209T105109_TCI.tif",
	"T31TCJ_20230211T104151_TCI.tif",
	"T31TCJ_20230212T110059_TCI.tif",
	"T31TCJ_20230214T105141_TCI.tif",
	"T31TCJ_20230216T104019_TCI.tif",
	"T31TCJ_20230217T110121_TCI.tif",
	"T31TCJ_20230219T105009_TCI.tif",
	"T31TCJ_20230221T104041_TCI.tif",
	"T31TCJ_20230222T105949_TCI.tif",
	"T31TCJ_20230224T105031_TCI.tif",
	"T31TCJ_20230226T103919_TCI.tif",
	"T31TCJ_20230227T110011_TCI.tif",
	"T31TCJ_20230301T104859_TCI.tif",
	"T31TCJ_20230303T103931_TCI.tif",
	"T31TCJ_20230304T105849_TCI.tif",
	"T31TCJ_20230306T104921_TCI.tif",
	"T31TCJ_20230308T103809_TCI.tif",
	"T31TCJ_20230309T105851_TCI.tif",
	"T31TCJ_20230311T104749_TCI.tif",
	"T31TCJ_20230313T103821_TCI.tif",
	"T31TCJ_20230314T105739_TCI.tif",
	"T31TCJ_20230316T104801_TCI.tif",
	"T31TCJ_20230318T103659_TCI.tif",
	"T31TCJ_20230319T105751_TCI.tif",
	"T31TCJ_20230321T104649_TCI.tif",
	"T31TCJ_20230323T103711_TCI.tif",
	"T31TCJ_20230324T105639_TCI.tif",
	"T31TCJ_20230326T104701_TCI.tif",
	"T31TCJ_20230328T103639_TCI.tif",
	"T31TCJ_20230329T105631_TCI.tif",
	"T31TCJ_20230331T104629_TCI.tif",
	"T31TCJ_20230402T103621_TCI.tif",
	"T31TCJ_20230403T105629_TCI.tif",
	"T31TCJ_20230405T105031_TCI.tif",
	"T31TCJ_20230407T103629_TCI.tif",
	"T31TCJ_20230408T105621_TCI.tif",
	"T31TCJ_20230410T104619_TCI.tif",
	"T31TCJ_20230412T103621_TCI.tif",
	"T31TCJ_20230413T105619_TCI.tif",
	"T31TCJ_20230415T104621_TCI.tif",
	"T31TCJ_20230417T103629_TCI.tif",
	"T31TCJ_20230418T105621_TCI.tif",
	"T31TCJ_20230420T104619_TCI.tif",
	"T31TCJ_20230422T103621_TCI.tif",
	"T31TCJ_20230423T105619_TCI.tif",
	"T31TCJ_20230425T104621_TCI.tif",
	"T31TCJ_20230427T103629_TCI.tif",
	"T31TCJ_20230428T105621_TCI.tif",
	"T31TCJ_20230430T104619_TCI.tif",
	"T31TCJ_20230502T103621_TCI.tif",
	"T31TCJ_20230503T105619_TCI.tif",
	"T31TCJ_20230505T104621_TCI.tif",
	"T31TCJ_20230507T103629_TCI.tif",
	"T31TCJ_20230508T105621_TCI.tif",
	"T31TCJ_20230510T104629_TCI.tif",
	"T31TCJ_20230512T103621_TCI.tif",
	"T31TCJ_20230513T105619_TCI.tif",
	"T31TCJ_20230515T104621_TCI.tif",
	"T31TCJ_20230517T103629_TCI.tif",
	"T31TCJ_20230518T105621_TCI.tif",
	"T31TCJ_20230520T104629_TCI.tif",
	"T31TCJ_20230522T103631_TCI.tif",
	"T31TCJ_20230523T105629_TCI.tif",
	"T31TCJ_20230525T105031_TCI.tif",
	"T31TCJ_20230527T103629_TCI.tif",
	"T31TCJ_20230528T105621_TCI.tif",
	"T31TCJ_20230530T104629_TCI.tif",
	"T31TCJ_20230601T104021_TCI.tif",
	"T31TCJ_20230602T105629_TCI.tif",
	"T31TCJ_20230604T104621_TCI.tif",
	"T31TCJ_20230606T103629_TCI.tif",
	"T31TCJ_20230607T105621_TCI.tif",
	"T31TCJ_20230609T104629_TCI.tif",
	"T31TCJ_20230611T103631_TCI.tif",
	"T31TCJ_20230612T105629_TCI.tif",
	"T31TCJ_20230614T105031_TCI.tif",
	"T31TCJ_20230616T103629_TCI.tif",
	"T31TCJ_20230617T105621_TCI.tif",
	"T31TCJ_20230619T104629_TCI.tif",
	"T31TCJ_20230621T103631_TCI.tif",
	"T31TCJ_20230622T105629_TCI.tif",
	"T31TCJ_20230624T104621_TCI.tif",
	"T31TCJ_20230626T103629_TCI.tif",
	"T31TCJ_20230627T105621_TCI.tif",
	"T31TCJ_20230629T104629_TCI.tif",
	"T31TCJ_20230701T103631_TCI.tif",
	"T31TCJ_20230702T105629_TCI.tif",
	"T31TCJ_20230704T104621_TCI.tif",
	"T31TCJ_20230706T103629_TCI.tif",
	"T31TCJ_20230707T105621_TCI.tif",
	"T31TCJ_20230709T104629_TCI.tif",
	"T31TCJ_20230711T103631_TCI.tif",
	"T31TCJ_20230712T105629_TCI.tif",
	"T31TCJ_20230714T105031_TCI.tif",
	"T31TCJ_20230716T103629_TCI.tif",
	"T31TCJ_20230717T105621_TCI.tif",
	"T31TCJ_20230719T104629_TCI.tif",
	"T31TCJ_20230721T103631_TCI.tif",
	"T31TCJ_20230722T105629_TCI.tif",
	"T31TCJ_20230724T104621_TCI.tif",
	"T31TCJ_20230726T103629_TCI.tif",
	"T31TCJ_20230727T105621_TCI.tif",
	"T31TCJ_20230729T104629_TCI.tif",
	"T31TCJ_20230731T103631_TCI.tif",
	"T31TCJ_20230801T105629_TCI.tif",
	"T31TCJ_20230803T104631_TCI.tif",
	"T31TCJ_20230805T103629_TCI.tif",
	"T31TCJ_20230806T105621_TCI.tif",
	"T31TCJ_20230808T104629_TCI.tif",
	"T31TCJ_20230813T105031_TCI.tif",
	"T31TCJ_20230815T103629_TCI.tif",
	"T31TCJ_20230816T105631_TCI.tif",
	"T31TCJ_20230818T104629_TCI.tif",
	"T31TCJ_20230820T103631_TCI.tif",
	"T31TCJ_20230821T105629_TCI.tif",
	"T31TCJ_20230823T104631_TCI.tif",
	"T31TCJ_20230825T103629_TCI.tif",
	"T31TCJ_20230826T105631_TCI.tif",
	"T31TCJ_20230828T104629_TCI.tif",
	"T31TCJ_20230830T103631_TCI.tif",
	"T31TCJ_20230831T105629_TCI.tif",
	"T31TCJ_20230902T104631_TCI.tif",
	"T31TCJ_20230904T103629_TCI.tif",
	"T31TCJ_20230905T105621_TCI.tif",
	"T31TCJ_20230907T104629_TCI.tif",
	"T31TCJ_20230909T103631_TCI.tif",
	"T31TCJ_20230910T105629_TCI.tif",
	"T31TCJ_20230912T104631_TCI.tif",
	"T31TCJ_20230914T103639_TCI.tif",
	"T31TCJ_20230915T105701_TCI.tif",
	"T31TCJ_20230917T104639_TCI.tif",
	"T31TCJ_20230919T103721_TCI.tif",
	"T31TCJ_20230920T105639_TCI.tif",
	"T31TCJ_20230922T104741_TCI.tif",
	"T31TCJ_20230924T103659_TCI.tif",
	"T31TCJ_20230925T105801_TCI.tif",
	"T31TCJ_20230927T104719_TCI.tif",
	"T31TCJ_20230929T103821_TCI.tif",
	"T31TCJ_20230930T105749_TCI.tif",
	"T31TCJ_20231002T104841_TCI.tif",
	"T31TCJ_20231004T103809_TCI.tif",
	"T31TCJ_20231005T105911_TCI.tif",
	"T31TCJ_20231007T104829_TCI.tif",
	"T31TCJ_20231009T103931_TCI.tif",
	"T31TCJ_20231010T105859_TCI.tif",
	"T31TCJ_20231012T104951_TCI.tif",
	"T31TCJ_20231014T103919_TCI.tif",
	"T31TCJ_20231015T110011_TCI.tif",
	"T31TCJ_20231017T104939_TCI.tif",
	"T31TCJ_20231019T104041_TCI.tif",
	"T31TCJ_20231020T110009_TCI.tif",
	"T31TCJ_20231022T105101_TCI.tif",
	"T31TCJ_20231024T104019_TCI.tif",
	"T31TCJ_20231025T110131_TCI.tif",
	"T31TCJ_20231027T105039_TCI.tif",
	"T31TCJ_20231029T104141_TCI.tif",
	"T31TCJ_20231030T110109_TCI.tif",
	"T31TCJ_20231101T105201_TCI.tif",
	"T31TCJ_20231103T104119_TCI.tif",
	"T31TCJ_20231104T110221_TCI.tif",
	"T31TCJ_20231106T105139_TCI.tif",
	"T31TCJ_20231108T104241_TCI.tif",
	"T31TCJ_20231109T110159_TCI.tif",
	"T31TCJ_20231111T105301_TCI.tif",
	"T31TCJ_20231113T104209_TCI.tif",
	"T31TCJ_20231114T110311_TCI.tif",
	"T31TCJ_20231116T105229_TCI.tif",
	"T31TCJ_20231118T104321_TCI.tif",
	"T31TCJ_20231119T110249_TCI.tif",
	"T31TCJ_20231121T105341_TCI.tif",
	"T31TCJ_20231124T110351_TCI.tif",
	"T31TCJ_20231126T105309_TCI.tif",
	"T31TCJ_20231128T104401_TCI.tif",
	"T31TCJ_20231129T110319_TCI.tif",
	"T31TCJ_20231201T105411_TCI.tif",
	"T31TCJ_20231203T104319_TCI.tif",
	"T31TCJ_20231204T110421_TCI.tif",
	"T31TCJ_20231206T105339_TCI.tif",
	"T31TCJ_20231208T104421_TCI.tif",
	"T31TCJ_20231209T110349_TCI.tif",
	"T31TCJ_20231211T105431_TCI.tif",
	"T31TCJ_20231213T104339_TCI.tif",
	"T31TCJ_20231214T110441_TCI.tif",
	"T31TCJ_20231216T105349_TCI.tif",
	"T31TCJ_20231218T104441_TCI.tif",
	"T31TCJ_20231219T110359_TCI.tif",
	"T31TCJ_20231221T105441_TCI.tif",
	"T31TCJ_20231223T104349_TCI.tif",
	"T31TCJ_20231224T110451_TCI.tif",
	"T31TCJ_20231226T105359_TCI.tif",
	"T31TCJ_20231228T104441_TCI.tif",
	"T31TCJ_20231229T110359_TCI.tif",
	"T31TCJ_20231231T105441_TCI.tif"}

func main() {
	var project, topic string
	var dstPrefix string
	var srcPrefix string
	var limit int
	var tilesize int
	var imagesize int

	flag.StringVar(&project, "project", os.Getenv("GCPPROJECT"), "GCP project")
	flag.StringVar(&topic, "topic", os.Getenv("MYNAME"), "PubSub topic")
	flag.StringVar(&dstPrefix, "dstPrefix", "gs://"+os.Getenv("BUCKETNAME")+"/results/", "Destination prefix")
	flag.StringVar(&srcPrefix, "srcPrefix", "/vsigs/tb-be-bigdata/t31tcj/", "Source prefix")
	flag.IntVar(&limit, "limit", 2, "Limit number of tiles")
	flag.IntVar(&tilesize, "tilesize", 512, "Tile size")
	flag.IntVar(&imagesize, "imagesize", 10980, "Image size")

	flag.Parse()
	ctx := context.Background()

	if project == "" {
		log.Fatal("-project or $GCPPROJECT is required")
	}
	if strings.HasPrefix(dstPrefix, "gs:///") {
		log.Fatal("-dstPrefix or $BUCKETNAME is required")
	}
	if topic == "" {
		log.Fatal("-topic or $MYNAME is required")
	}

	psCl, err := pubsub.NewClient(context.Background(), project)
	if err != nil {
		panic(err)
	}
	topicObj := psCl.Topic(topic)

	for i := range inputs {
		inputs[i] = srcPrefix + inputs[i]
	}

	var wg sync.WaitGroup

	n := 0
	var totalErrors uint64
outer:
	for x := 0; x < imagesize-1; x += tilesize {
		xsize := tilesize
		if x+tilesize > imagesize {
			xsize = imagesize - x
		}
		for y := 0; y < imagesize-1; y += tilesize {
			if limit > 0 && n >= limit {
				break outer
			}
			n++
			ysize := tilesize
			if y+tilesize > imagesize {
				ysize = imagesize - y
			}
			mrequest := MRequest{
				Datasets:    inputs,
				Window:      [4]int{x, y, xsize, ysize},
				Destination: fmt.Sprintf("%stile%d-%d.tif", dstPrefix, x, y),
			}
			mreqb, _ := json.Marshal(mrequest)
			//log.Printf("%v", mrequest.Window)
			//continue
			wg.Add(1)
			res := topicObj.Publish(ctx, &pubsub.Message{
				Data: mreqb,
			})
			go func(i int, res *pubsub.PublishResult) {
				defer wg.Done()
				id, err := res.Get(ctx)
				if err != nil {
					log.Printf("Failed to publish: %v", err)
					atomic.AddUint64(&totalErrors, 1)
					return
				}
				log.Printf("Published tile %v; msg ID: %v\n", mrequest.Window, id)

			}(n, res)
		}
	}
	wg.Wait()

	if totalErrors > 0 {
		log.Printf("%d of %d messages did not publish successfully", totalErrors, n)
	} else {
		log.Printf("%d messages published successfully", n)
	}
}
