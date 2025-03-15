# Golang Download Progress Example

This project demonstrates a simple Golang program that downloads files from the internet while displaying a progress bar in the terminal. It leverages the `uiprogress` package to visualize download progress and `go-humanize` to format byte sizes and durations.

## Features

- **File Downloading:** Downloads files from specified URLs.
- **Progress Bar:** Displays real-time progress using a customizable progress bar.
- **Time Tracking:** Shows elapsed time during the download.
- **User-Friendly Status Updates:** Indicates different download statuses (e.g., preparing, downloading, done).

## Requirements

- [Go](https://golang.org/) (version 1.22 or later recommended)

## Installation

1. **Clone the Repository:**

   ```bash
   git clone https://github.com/angelo1121/downloader.git
   cd downloader
   ```

   The repository already includes a `go.mod` file, so you don't need to initialize a new module.

2. **Download Dependencies:**

   ```bash
   go mod download
   ```

   Ensure your module dependencies are up-to-date by running:

   ```bash
   go mod tidy
   ```

## Usage

Compile and run the program using the Go command:

```bash
go run main.go
```

By default, the program will download a PDF file from:
```
https://freetestdata.com/wp-content/uploads/2022/11/Free_Test_Data_10.5MB_PDF.pdf
```
and save it as `test1.pdf` in your working directory.

## Customization

- **Adding More Downloads:**
  Modify the `downloaders` slice in the `main` function to include additional URL and filename pairs.

- **Adjusting Progress Bar Settings:**
  The progress bar's behavior can be adjusted by modifying the `refreshRate` and `barLength` constants in the source code.

- **Error Handling:**
  The program outputs error messages to the console for issues such as HTTP request failures or file write errors.
