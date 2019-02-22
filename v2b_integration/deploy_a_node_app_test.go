package v2b_integration_test

import (
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cloudfoundry/libbuildpack/cutlass"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("CF NodeJS Buildpack", func() {
	var app *cutlass.App
	AfterEach(func() {
		if app != nil {
			//app.Destroy()
		}
		app = nil
	})

	Describe("nodeJS versions", func() {
		Context("when specifying a range for the nodeJS version in the package.json", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join("testdata", "node_version_range"))
			})

			XIt("resolves to a nodeJS version successfully", func() {
				PushAppAndConfirm(app)

				Eventually(cutlass.StripColor(app.Stdout.String())).Should(MatchRegexp("NodeJS 6\\.\\d+\\.\\d+"))
				Expect(app.GetBody("/")).To(ContainSubstring("Hello, World!"))
			})
		})

		Context("when specifying a version 6 for the nodeJS version in the package.json", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join("testdata", "node_version_6"))
			})

			XIt("resolves to a nodeJS version successfully", func() {
				PushAppAndConfirm(app)

				Eventually(cutlass.StripColor(app.Stdout.String())).Should(MatchRegexp("NodeJS 6\\.\\d+\\.\\d+"))
				Expect(app.GetBody("/")).To(ContainSubstring("Hello, World!"))

				if ApiHasTask() {
					By("running a task", func() {
						By("can find node in the container", func() {
							command := exec.Command("cf", "run-task", app.Name, "echo \"RUNNING A TASK: $(node --version)\"")
							_, err := command.Output()
							Expect(err).To(BeNil())

							Eventually(func() string {
								return app.Stdout.String()
							}, "30s").Should(MatchRegexp("RUNNING A TASK: v6\\.\\d+\\.\\d+"))
						})
					})
				}
			})
		})

		Context("when not specifying a nodeJS version in the package.json", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join("testdata", "without_node_version"))
			})

			FIt("resolves to the default nodeJS version successfully", func() {
				PushAppAndConfirm(app)

				Eventually(cutlass.StripColor(app.Stdout.String())).Should(MatchRegexp("NodeJS 6\\.\\d+\\.\\d+"))
				Expect(app.GetBody("/")).To(ContainSubstring("Hello, World!"))
			})
		})

		Context("with an unreleased nodejs version", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join("testdata", "unreleased_node_version"))
			})

			XIt("displays a nice error message and gracefully fails", func() {
				Expect(app.Push()).ToNot(BeNil())

				Eventually(app.Stdout.String, 2*time.Second).Should(MatchRegexp(`no valid dependencies for node 9000\.0\.0, and .* in`))
				Expect(app.ConfirmBuildpack(buildpackVersion)).To(Succeed())
			})
		})

		Context("with an unsupported, but released, nodejs version", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join("testdata", "unsupported_node_version"))
			})

			XIt("displays a nice error messages and gracefully fails", func() {
				Expect(app.Push()).ToNot(BeNil())

				Eventually(app.Stdout.String, 2*time.Second).Should(MatchRegexp(`no valid dependencies for node 4\.1\.1, and .* in`))
				Expect(app.ConfirmBuildpack(buildpackVersion)).To(Succeed())
			})
		})
	})

	Context("with no Procfile and OPTIMIZE_MEMORY=true", func() {
		BeforeEach(func() {
			app = cutlass.New(filepath.Join("testdata", "simple_app"))
			app.SetEnv("OPTIMIZE_MEMORY", "true")
		})

		XIt("is running with autosized max_old_space_size", func() {
			PushAppAndConfirm(app)

			Expect(app.GetBody("/")).To(ContainSubstring("NodeOptions: --max_old_space_size=96"))
		})
	})

	Context("with no Procfile and OPTIMIZE_MEMORY is unset", func() {
		BeforeEach(func() {
			app = cutlass.New(filepath.Join("testdata", "simple_app"))
		})

		XIt("is not running with autosized max_old_space_size", func() {
			PushAppAndConfirm(app)

			Expect(app.GetBody("/")).To(ContainSubstring("NodeOptions: undefined"))
		})

		Context("a nvmrc file that takes precedence over package.json", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join("testdata", "simple_app_with_nvmrc"))
			})

			XIt("deploys", func() {
				PushAppAndConfirm(app)

				Expect(app.GetBody("/")).To(ContainSubstring("NodeOptions: undefined"))
			})
		})
	})

	Describe("Vendored Dependencies", func() {
		Context("with an app that has vendored dependencies", func() {
			It("deploys", func() {
				app = cutlass.New(filepath.Join("testdata", "vendored_dependencies"))
				app.SetEnv("BP_DEBUG", "true")
				PushAppAndConfirm(app)

				By("does not output protip that recommends user vendors dependencies", func() {
					Expect(app.Stdout.String()).ToNot(MatchRegexp("PRO TIP:(.*) It is recommended to vendor the application's Node.js dependencies"))
				})

				if !cutlass.Cached {
					By("with an uncached buildpack", func() {
						By("successfully deploys and includes the dependencies", func() {
							Expect(app.GetBody("/")).To(ContainSubstring("0000000005"))
							Eventually(app.Stdout.String).Should(ContainSubstring("Download [https://"))
						})
					})
				}

				if cutlass.Cached {
					By("with a cached buildpack", func() {
						By("deploys without hitting the internet", func() {
							Expect(app.GetBody("/")).To(ContainSubstring("0000000005"))
							Eventually(app.Stdout.String).Should(ContainSubstring("Copy [/tmp/buildpacks/"))
						})
					})
				}
			})

			AssertNoInternetTraffic("vendored_dependencies")
		})

		Context("Vendored Depencencies with node module binaries", func() {
			BeforeEach(func() {
				if !ApiSupportsSymlinks() {
					Skip("Requires api symlink support")
				}
			})

			It("deploys", func() {
				app = cutlass.New(filepath.Join("testdata", "vendored_dependencies_with_binaries"))
				app.SetEnv("BP_DEBUG", "true")
				PushAppAndConfirm(app)
			})
		})

		Context("with an app with a yarn.lock and vendored dependencies", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join("testdata", "with_yarn_vendored"))
				app.SetEnv("BP_DEBUG", "true")
			})

			PIt("deploys without hitting the internet", func() {
				PushAppAndConfirm(app)

				Expect(filepath.Join(app.Path, "node_modules")).To(BeADirectory())
				Eventually(app.Stdout.String).Should(ContainSubstring("Running yarn in offline mode"))
				Expect(app.GetBody("/microtime")).To(MatchRegexp("native time: \\d+\\.\\d+"))
			})

			AssertNoInternetTraffic("with_yarn_vendored")
		})

		Context("with an incomplete node_modules directory", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join("testdata", "incomplete_node_modules"))
			})

			It("downloads missing dependencies from package.json", func() {
				PushAppAndConfirm(app)
				Expect(filepath.Join(app.Path, "node_modules")).To(BeADirectory())
				Expect(filepath.Join(app.Path, "node_modules", "hashish")).ToNot(BeADirectory())
				Expect(app.Files("app/node_modules")).To(ContainElement("app/node_modules/hashish"))
				Expect(app.Files("app/node_modules")).To(ContainElement("app/node_modules/express"))
			})
		})

		Context("with an incomplete package.json", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join("testdata", "incomplete_package_json"))
			})

			It("does not overwrite the vendored modules not listed in package.json", func() {
				PushAppAndConfirm(app)
				Expect(app.Files("app/node_modules")).To(ContainElement("app/node_modules/leftpad"))
				Expect(app.Files("app/node_modules")).To(ContainElement("app/node_modules/hashish"))
			})
		})
	})

	Describe("No Vendored Dependencies", func() {
		Context("with an app with no vendored dependencies", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join("testdata", "no_vendored_dependencies"))
				app.SetEnv("BP_DEBUG", "true")
			})

			It("successfully deploys and vendors the dependencies", func() {
				PushAppAndConfirm(app)

				Expect(filepath.Join(app.Path, "node_modules")).ToNot(BeADirectory())

				depsDir, err := app.GetBody("/deps_dir")
				Expect(err).ToNot(HaveOccurred())
				Expect(app.Files(depsDir)).To(ContainElement(depsDir + "/0/node_modules"))

				Expect(app.GetBody("/")).To(ContainSubstring("Hello, World!"))

				By("outputs protip that recommends user vendors dependencies", func() {
					Eventually(app.Stdout.String).Should(MatchRegexp("PRO TIP:(.*) It is recommended to vendor the application's Node.js dependencies"))
				})
			})

			AssertUsesProxyDuringStagingIfPresent("no_vendored_dependencies")
		})

		PContext("with an app with a yarn.lock file", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join("testdata", "with_yarn"))
				app.SetEnv("BP_DEBUG", "true")
			})

			PIt("successfully deploys and vendors the dependencies via yarn", func() {
				PushAppAndConfirm(app)

				Expect(filepath.Join(app.Path, "node_modules")).ToNot(BeADirectory())

				Eventually(app.Stdout.String).Should(ContainSubstring("Running yarn in online mode"))

				depsDir, err := app.GetBody("/deps_dir")
				Expect(err).ToNot(HaveOccurred())
				Expect(app.Files(depsDir)).To(ContainElement(depsDir + "/0/node_modules"))

				Expect(app.GetBody("/")).To(ContainSubstring("Hello, World!"))
			})

			AssertUsesProxyDuringStagingIfPresent("with_yarn")
		})

		PContext("with an app with an out of date yarn.lock", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join("testdata", "out_of_date_yarn_lock"))
			})

			PIt("warns that yarn.lock is out of date", func() {
				PushAppAndConfirm(app)
				Eventually(app.Stdout.String).Should(ContainSubstring("yarn.lock is outdated"))
			})
		})

		Context("with an app with pre and post scripts", func() {
			BeforeEach(func() {
				app = cutlass.New(filepath.Join("testdata", "pre_post_commands"))
			})

			PIt("runs the scripts through npm run", func() {
				PushAppAndConfirm(app)
				Eventually(app.Stdout.String).Should(ContainSubstring("Running heroku-prebuild (npm)"))
				Eventually(app.Stdout.String).Should(ContainSubstring("Running heroku-postbuild (npm)"))
				Expect(app.GetBody("/")).To(ContainSubstring("Text: Hello Buildpacks Team"))
				Expect(app.GetBody("/")).To(ContainSubstring("Text: Goodbye Buildpacks Team"))
			})

			PIt("runs the postinstall script in the app directory", func() {
				PushAppAndConfirm(app)
				Eventually(app.Stdout.String, 2*time.Second).Should(ContainSubstring("Current dir: /tmp/app"))
			})
		})
	})

	Describe("NODE_HOME and NODE_ENV", func() {
		BeforeEach(func() {
			if !cutlass.Cached {
				Skip("running uncached tests")
			}
			app = cutlass.New(filepath.Join("testdata", "logenv"))
		})

		XIt("sets the NODE_HOME to correct value", func() {
			PushAppAndConfirm(app)
			Eventually(app.Stdout.String).Should(MatchRegexp("Writing NODE_HOME"))

			body, err := app.GetBody("/")
			Expect(err).To(BeNil())
			Expect(body).To(MatchRegexp(`"NODE_HOME":"[^"]*/node"`))
			Expect(body).To(ContainSubstring(`"NODE_ENV":"production"`))
			Expect(body).To(ContainSubstring(`"MEMORY_AVAILABLE":"128"`))
		})
	})

	Describe("System CA Store", func() {
		BeforeEach(func() {
			app = cutlass.New(filepath.Join("testdata", "use-openssl-ca"))
			app.SetEnv("SSL_CERT_FILE", "cert.pem")
		})

		XIt("uses the system CA store (or env)", func() {
			PushAppAndConfirm(app)
			Expect(app.GetBody("/")).To(ContainSubstring("Response over self signed https"))
		})
	})
})
