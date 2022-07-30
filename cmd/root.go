package cmd

import (
	"bufio"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var directory string
var image string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "exifLooter",
	Short: "ExifLooter finds GeoLocation Metadata and display",
	Long:  `ExifLooter finds GeoLocation Metadata and display. You can use with pipe and flags`,
	Run:   analyzeFlags,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {

	rootCmd.PersistentFlags().BoolP("pipe", "p", false, "Pipe with other scripts")

	rootCmd.PersistentFlags().BoolP("remove", "r", false, "Remove metadata from Image")

	rootCmd.PersistentFlags().StringP("image", "i", "", "Specify a image for Analyzing")

	rootCmd.PersistentFlags().StringP("directory", "d", "", "Specify a directory for Analyzing")
}

func analyzeFlags(cmd *cobra.Command, _ []string) {
	i, err := cmd.Flags().GetString("image")
	if err != nil {
		log.Fatal(err)
	}

	image = i

	d, err := cmd.Flags().GetString("directory")
	if err != nil {
		log.Fatal(err)
	}

	directory = d

	p, err := cmd.Flags().GetBool("pipe")
	if err != nil {
		log.Fatal(err)
	}

	rmv, err := cmd.Flags().GetBool("remove")
	if err != nil {
		log.Fatal(err)
	}

	if len(directory) != 0 {
		analyzeDirectory(cmd)
	} else if rmv {
		if len(directory) != 0{
			removeMetadataDirectory(cmd)
		} else{
			removeMetadata(cmd, image, false)
		}
	} else if len(image) != 0 {
		analyzeImages(cmd, image, false)
	} else if p {
		pipeImages()
	}
}

func analyzeImages(cmd *cobra.Command, args string, inDir bool) {
	if !inDir {
		img, err := cmd.Flags().GetString("image")
		if err != nil {
			log.Fatal(err)
		}

		out, err := exec.Command("exiftool", img).Output()

		if err != nil {
			log.Fatal(err)
		}

		// fmt.Println(string(out))
		parseOutput(string(out))
	} else {
		out, err := exec.Command("exiftool", directory+args).Output()

		if err != nil {
			log.Fatal(err)
		}

		// fmt.Println(string(out))
		parseOutput(string(out))
	}

}

func analyzeDirectory(cmd *cobra.Command) {
	files, err := ioutil.ReadDir(directory)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		} else {
			analyzeImages(cmd, file.Name(), true)
		}
	}
}

func removeMetadata(cmd *cobra.Command, args string, inDir bool) {
	if !inDir {
		img, err := cmd.Flags().GetString("image")
		if err != nil {
			log.Fatal(err)
		}

		out, err := exec.Command("exiftool", "-all=", img).Output()

		if err != nil {
			log.Fatal(err)
		}

		// fmt.Println(string(out))
		parseOutput(string(out))
	} else {
		out, err := exec.Command("exiftool", "-all=", directory+args).Output()

		if err != nil {
			log.Fatal(err)
		}

		// fmt.Println(string(out))
		parseOutput(string(out))
	}
}

func removeMetadataDirectory(cmd *cobra.Command){
	files, err := ioutil.ReadDir(directory)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		} else {
			removeMetadata(cmd, file.Name(), true)
		}
	}
}

func parseOutput(out string) {
	scanner := bufio.NewScanner(strings.NewReader(out))
	var flag bool
	for scanner.Scan() {
		txt := scanner.Text()
		key := strings.Split(txt, ":")
		key[0] = standardizeSpaces(key[0])
		if strings.Contains(key[0], "GPS") {
			flag = true
			color.Yellow(key[0] + ":" + key[1])

		}
	}

	if !flag {
		color.Green("These image/images not Vulnerable")
	} else {
		color.Red("EXIF Geolocation Data Not Stripped From Uploaded Images")
	}
}

func parseOutputPipe(out string, fname string) {
	vuln := false
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		txt := scanner.Text()
		key := strings.Split(txt, ":")
		key[0] = standardizeSpaces(key[0])
		if strings.Contains(key[0], "GPS") {
			color.Red("EXIF Geolocation Data Not Stripped From Uploaded Images")
			color.Yellow(key[0] + ":" + key[1])
			vuln = true
		}
	}
	if !vuln {
		color.Green(fname + " is not Vulnerable")
	}
}

func pipeImages() {
	if _, err := os.Stat("images/"); os.IsNotExist(err) {
		err := os.Mkdir("images/", 777)
		if err != nil {
			log.Fatal(err)
		}
	}
	// Check for stdin input
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		color.Red("No urls detected. Hint: cat urls.txt | exifLooter -p")
		os.Exit(1)
	}

	results := make(chan string, 8)

	go func() {
		// get each line of stdin, push it to the work channel
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			line := s.Text()
			//fmt.Println(line)

			response, e := http.Get(line)
			if e != nil {
				log.Fatal(e)
			}

			defer func(Body io.ReadCloser) {
				err := Body.Close()
				if err != nil {
					log.Fatal(err)
				}
			}(response.Body)

			//open a file for writing
			//fmt.Println(line)

			file, err := os.Create("images/" + strconv.Itoa(int(getTimestamp())) + "." + getFileExtensionFromUrl(line))
			if err != nil {
				log.Fatal(err)
			}

			defer func(file *os.File) {
				err := file.Close()
				if err != nil {
					log.Fatal(err)
				}
			}(file)

			// Use io.Copy to just dump the response body to the file. This supports huge files

			_, err = io.Copy(file, response.Body)
			if err != nil {
				log.Fatal(err)
			}
			//fmt.Println(b)

			//fmt.Println("Success!")

			checkValidImage()

		}
		if err := s.Err(); err != nil {
			log.Println("reading standard input:", err)
		}
		close(results)
	}()

	w := bufio.NewWriter(os.Stdout)
	defer func(w *bufio.Writer) {
		err := w.Flush()
		if err != nil {
			log.Fatal(err)
		}
	}(w)

	for res := range results {
		log.Println(w, res)
	}

	files, err := ioutil.ReadDir("images/")
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		_, err := os.ReadFile("images/" + f.Name())
		if err != nil {
			log.Fatal(err)
		}

		out, err := exec.Command("exiftool", "images/"+f.Name()).Output()

		if err != nil {
			log.Fatal(err)
		}

		// fmt.Println(string(out))
		parseOutputPipe(string(out), f.Name())

	}

}

func getFileExtensionFromUrl(rawUrl string) string {
	u, err := url.Parse(rawUrl)
	if err != nil {
		log.Fatal(err)
	}
	pos := strings.LastIndex(u.Path, ".")
	if pos == -1 {
		return ""
	}
	return u.Path[pos+1 : len(u.Path)]
}

func standardizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func getTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func checkValidImage() bool {
	files, err := ioutil.ReadDir("images/")
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		file, err := os.ReadFile("images/" + f.Name())
		if err != nil {
			return false
		}

		if !strings.Contains(http.DetectContentType(file), "image/") {
			err := os.Remove("images/" + f.Name())
			if err != nil {
				return false
			}
		}
	}

	return false
}
