package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Creates a new subtitle file from the inputfile, but with all the time fields
// incremented by 'seconds' seconds (decremented when negative).
//
// The name of the new file is identical to the old one, but prepended with "{+x.xx_Sec}_".
func main() {
	if len(os.Args) < 3 {
		log.Fatal("\nUsage: submod inputfile seconds\n",
			"The following arguments are required: inputfile, seconds")
	}

	var inputfile string = os.Args[1]
	var outputfile string
	var deleted_subs int
	var seconds float64

	seconds, err := strconv.ParseFloat(os.Args[2], 64)
	if err != nil {
		log.Fatal("\nUsage: submod inputfile seconds\n" +
			"The seconds field should be numeric.")
	}

	if strings.HasSuffix(inputfile, ".srt") {
		outputfile = name_output(inputfile, seconds)
		deleted_subs = convert_srt(inputfile, outputfile, seconds)
	} else if strings.HasSuffix(inputfile, ".vtt") {
		outputfile = name_output(inputfile, seconds)
		deleted_subs = convert_vtt(inputfile, outputfile, seconds)
	} else {
		fmt.Println("Please specify either an .srt or .vtt file as input.")
		os.Exit(1)
	}

	status(deleted_subs, outputfile)
}

// Determines the name of the outputfile based on the inputfile and seconds;
// the name of the new file is identical to the old one, but prepended with "{+x.xx_Sec}_".
//
// However, if the file has already been processed by submod before, we simply change
// the 'increment number' x, instead of prepending "{+x.xx_Sec}_" a second time.
// This way we can conveniently process files multiple times, and still have sensible names.
func name_output(inputfile string, seconds float64) string {
	// Regex to check if the inputfile was previously processed by submod:
	proc, err := regexp.Compile(`\{[+-]\d+\.\d+_Sec\}_`)
	if err != nil {
		log.Fatal(err)
	}

	var processed bool = proc.MatchString(inputfile)
	var placeholder string
	var incr float64

	// Inputfile was processed by submod previously:
	if processed {
		// Regex for extracting the increment number from the inputfile:
		re, err := regexp.Compile(`[+-]\d+\.\d+`)
		if err != nil {
			log.Fatal(err)
		}

		// FindString extracts the leftmost occurrence of 're'
		var number string = re.FindString(inputfile)

		incr, err = strconv.ParseFloat(number, 64)
		if err != nil {
			log.Fatal("\nError processing seconds for filename:\n", err)
		}
		incr += seconds

		// FindStringIndex returns the start
		// to end indices of the leftmost occurrence of proc as a slice,
		// which we then use to replace proc with the format:
		index := proc.FindStringIndex(inputfile)
		placeholder = "{%.2f_Sec}_" + inputfile[index[1]:]
	} else {
		incr = seconds
		placeholder = "{%.2f_Sec}_" + inputfile
	}

	if incr >= 0 {
		placeholder = "{+" + placeholder[1:]
	}

	var outputfile string = fmt.Sprintf(placeholder, incr)

	return outputfile
}

// Loops through the given inputfile, modifies the lines consisting of the time encoding,
// writes everything back to outputfile, and returns the number of subtitles that were deleted.
//
// This function is identical to convert_srt,
// except that it uses '.' for the seconds field's decimal space.
//
// The subtitle files consist of a repetition of the following 3 lines:
//
// - Index-line: integer count indicating line number
// - Time-line: encoding the duration for which the subtitle appears
// - Sub-line: the actual subtitle to appear on-screen (1 or 2 lines)
//
// Example .vtt (Note: '.' for decimal spaces):
//
// 1
// 00:00:00.243 --> 00:00:02.110
// Previously on ...
//
// 2
// 00:00:03.802 --> 00:00:05.314
// Etc.
func convert_vtt(inputfile string, outputfile string, seconds float64) int {
	input, err := os.Open(inputfile)
	if err != nil {
		log.Fatal(err)
	}
	defer input.Close()

	output, err := os.Create(outputfile)
	if err != nil {
		log.Fatal(err)
	}
	defer output.Close()

	// Compile regex to find time-line:
	re, err := regexp.Compile(`\d\d:\d\d:\d\d\.\d\d\d`)
	if err != nil {
		log.Fatal(err)
	}

	var deleted_subs int = 0
	var skip bool = false

	// Iterate line by line over inputfile:
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {

		var old_line string = scanner.Text()
		var new_line string
		var time_line bool = re.MatchString(old_line)

		// Time-line: This is the line we need to modify
		if time_line {
			new_line = process_line(old_line, seconds)
			if new_line == "(DELETED)\n" {
				deleted_subs += 1
				skip = true
			}
		} else {
			// When skip = True, subtitles are shifted too far back
			// into the past (before the start of the movie),
			// so they are deleted:
			if skip {
				// Subtitles can be 1 or 2 lines; we should only update
				// skip when we have arrived at an empty line:
				if old_line == "" {
					skip = false
				}
				continue
			} else {
				new_line = old_line
			}
		}

		_, err = output.WriteString(new_line + "\n")
		if err != nil {
			log.Fatal(err)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return deleted_subs
}

// Loops through the given inputfile, modifies the lines consisting of the time encoding,
// writes everything back to outputfile, and returns the number of subtitles that were deleted.
//
// This function is identical to convert_vtt,
// except that it uses ',' for the seconds field's decimal space.
//
// The subtitle files consist of a repetition of the following 3 lines:
//
// - Index-line: integer count indicating line number
// - Time-line: encoding the duration for which the subtitle appears
// - Sub-line: the actual subtitle to appear on-screen (1 or 2 lines)
//
// Example .srt (Note: ',' for decimal spaces):
//
// 1
// 00:00:00,243 --> 00:00:02,110
// Previously on ...
//
// 2
// 00:00:03,802 --> 00:00:05,314
// Etc.
func convert_srt(inputfile string, outputfile string, seconds float64) int {
	input, err := os.Open(inputfile)
	if err != nil {
		log.Fatal(err)
	}
	defer input.Close()

	output, err := os.Create(outputfile)
	if err != nil {
		log.Fatal(err)
	}
	defer output.Close()

	// Compile regex to find time-line:
	re, err := regexp.Compile(`\d\d:\d\d:\d\d,\d\d\d`)
	if err != nil {
		log.Fatal(err)
	}

	var deleted_subs int = 0
	var skip bool = false

	// Iterate line by line over inputfile:
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {

		var old_line string = scanner.Text()
		var new_line string
		var time_line bool = re.MatchString(old_line)

		// Time-line: This is the line we need to modify
		if time_line {
			// We need '.' instead of ',' for floats!
			new_line = strings.Replace(old_line, ",", ".", 2)
			new_line = process_line(new_line, seconds)
			if new_line == "(DELETED)\n" {
				deleted_subs += 1
				skip = true
			} else {
				// Convert back to '.srt' style:
				new_line = strings.Replace(new_line, ".", ",", 2)
			}
		} else {
			// When skip = True, subtitles are shifted too far back
			// into the past (before the start of the movie),
			// so they are deleted:
			if skip {
				// Subtitles can be 1 or 2 lines; we should only update
				// skip when we have arrived at an empty line:
				if old_line == "" {
					skip = false
				}
				continue
			} else {
				new_line = old_line
			}
		}

		_, err = output.WriteString(new_line + "\n")
		if err != nil {
			log.Fatal(err)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return deleted_subs
}

// Process the given line by adding seconds to start and end time.
// (subtracting if seconds is negative)
//
// Example line:  '00:00:01.913 --> 00:00:04.328'
// Index:          01234567890123456789012345678
// Index by tens: (0)        10        20     (28)
func process_line(line string, seconds float64) string {
	var start string = line[0:12]
	start = process_time(start, seconds)

	var end string = line[17:29]
	end = process_time(end, seconds)

	if start == "(DELETED)\n" {
		if end == "(DELETED)\n" {
			line = "(DELETED)\n"
		} else {
			line = "00:00:00.000 --> " + end
		}
	} else {
		line = start + " --> " + end
	}

	return line
}

// Increment the given time_string by 'incr' seconds
//
// The time-string has the form '00:00:00.000',
// and converts to the following format string:
// "%02d:%02d:%06.3f"
func process_time(time_string string, incr float64) string {
	hrs, err := strconv.Atoi(time_string[0:2])
	if err != nil {
		log.Fatal("\nError processing hours:\n", err)
	}
	mins, err := strconv.Atoi(time_string[3:5])
	if err != nil {
		log.Fatal("\nError processing minutes:\n", err)
	}
	secs, err := strconv.ParseFloat(time_string[6:12], 64)
	if err != nil {
		log.Fatal("\nError processing seconds:\n", err)
	}

	var hr time.Duration = time.Duration(hrs) * time.Hour
	var min time.Duration = time.Duration(mins) * time.Minute
	var sec time.Duration = time.Duration(secs*1000) * time.Millisecond
	var delta time.Duration = time.Duration(incr*1000) * time.Millisecond
	var new_time time.Duration = hr + min + sec + delta

	// incr can be negative, so the new time could be too:
	if new_time >= 0 {
		// NOT casting to int64 might be problematic on 32 bit systems though:
		// when int is 32 bits wide, it can't hold the largest of time.Duration values (which are 64 bit)!
		// But this shouldn't be a problem for the small values we expect.
		hrs = int(new_time / time.Hour)
		mins = int((new_time % time.Hour) / time.Minute)
		secs = float64((new_time%time.Minute)/time.Millisecond) / 1000
		time_string = fmt.Sprintf("%02d:%02d:%06.3f", hrs, mins, secs)
	} else {
		// new_time < 0: the subtitles are now scheduled before the start
		// of the movie, so we can delete them:
		time_string = "(DELETED)\n"
	}

	return time_string
}

// Prints a status update for the user.
func status(deleted_subs int, outputfile string) {
	var text string
	if deleted_subs > 0 {
		if deleted_subs == 1 {
			text = "Success.\nOne subtitle was deleted at the beginning of the file."
		} else {
			text = "Success.\n" + strconv.Itoa(deleted_subs) +
				" subtitles were deleted at the beginning of the file."
		}
	} else {
		text = "Success."
	}

	fmt.Println(text)
	fmt.Println("Filename = ", outputfile)
}
