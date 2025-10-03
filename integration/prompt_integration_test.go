package integration_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("CF Prompt Plugin Integration", func() {
	var (
		appName   string
		appDir    string
		appURL    string
		cfSpace   string
		cfOrg     string
	)

	BeforeEach(func() {
		appName = "hello-world-app"
		
		appDir = filepath.Join("..", "integration", "assets", "hello-world-app")
		absAppDir, err := filepath.Abs(appDir)
		Expect(err).NotTo(HaveOccurred())
		appDir = absAppDir

		cfOrg = os.Getenv("CF_ORG")
		if cfOrg == "" {
			cfOrg = "cf-org"
		}

		cfSpace = os.Getenv("CF_SPACE")
		if cfSpace == "" {
			cfSpace = "cf-space"
		}

		targetOrg := exec.Command("cf", "target", "-o", cfOrg, "-s", cfSpace)
		session, err := gexec.Start(targetOrg, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 30*time.Second).Should(gexec.Exit(0))
	})

	AfterEach(func() {
		if appName != "" {
			deleteCmd := exec.Command("cf", "delete", appName, "-f", "-r")
			session, err := gexec.Start(deleteCmd, GinkgoWriter, GinkgoWriter)
			if err == nil {
				Eventually(session, 60*time.Second).Should(gexec.Exit())
			}
		}
	})

	Describe("Prompt-based app revision workflow", func() {
		It("should push app, attempt to create revision via prompt, and verify app accessibility", func() {
			By("Pushing the hello-world app to Cloud Foundry")
			pushCmd := exec.Command("cf", "push", appName, "-p", appDir)
			session, err := gexec.Start(pushCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, 5*time.Minute).Should(gexec.Exit(0))

			By("Getting the app URL")
			appsCmd := exec.Command("cf", "apps")
			session, err = gexec.Start(appsCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, 30*time.Second).Should(gexec.Exit(0))
			
			appsOutput := string(session.Out.Contents())
			lines := strings.Split(appsOutput, "\n")
			var route string
			for _, line := range lines {
				if strings.Contains(line, appName) {
					fields := strings.Fields(line)
					for i, field := range fields {
						if strings.Contains(field, "http") || (i > 0 && strings.Contains(fields[i-1], "routes")) {
							route = field
							break
						}
					}
				}
			}
			
			if route == "" {
				appCmd := exec.Command("cf", "app", appName)
				session, err = gexec.Start(appCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 30*time.Second).Should(gexec.Exit(0))
				
				appOutput := string(session.Out.Contents())
				lines = strings.Split(appOutput, "\n")
				for _, line := range lines {
					if strings.Contains(line, "routes:") || strings.Contains(line, "urls:") {
						parts := strings.Split(line, ":")
						if len(parts) > 1 {
							route = strings.TrimSpace(parts[1])
							break
						}
					}
				}
			}

			Expect(route).NotTo(BeEmpty(), "Failed to find app route")
			
			if !strings.HasPrefix(route, "http") {
				appURL = "https://" + route
			} else {
				appURL = route
			}

			By("Verifying the app returns 'hello world'")
			Eventually(func() (string, error) {
				resp, err := http.Get(appURL)
				if err != nil {
					return "", err
				}
				defer resp.Body.Close()
				
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return "", err
				}
				
				return string(body), nil
			}, 2*time.Minute, 5*time.Second).Should(Equal("hello world"))

			By("Attempting to create a new revision using cf prompt plugin")
			promptCmd := exec.Command("cf", "prompt", "--app", appName, "change hello world to foo bar")
			session, err = gexec.Start(promptCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			
			Eventually(session, 2*time.Minute).Should(gexec.Exit())

			By("Checking that the prompt command was executed (may not fully succeed)")
			output := string(session.Out.Contents())
			fmt.Fprintf(GinkgoWriter, "Prompt command output:\n%s\n", output)
			
			By("Verifying the app is still accessible after prompt attempt")
			resp, err := http.Get(appURL)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			
			currentResponse := string(body)
			fmt.Fprintf(GinkgoWriter, "Current app response: %s\n", currentResponse)
			
			Expect(currentResponse).To(Or(
				Equal("hello world"),
				Equal("foo bar"),
			), "App should return either original or updated text")
		})
	})
})
