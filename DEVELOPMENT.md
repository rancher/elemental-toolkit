# Tips and tricks for development

Some helpful tips and docs for development

## Enable video capture on virtualbox (packer, tests)

When building the images with packer or running the tests with virtualbox, there is a possibility of enabling video capture of the machine output for easy troubleshooting.
This is helful on situations when we cannot access the machine directly like in CI tests.

This is disabled by default but can be enabled so the output is captured. The CI is prepared to save the resulting artifacts if this is enabled.

For enabling it in tests you just need to set the env var `ENABLE_VIDEO_CAPTURE` to anything (true, on, whatever you like) and a capture.webm will be created and upload as an artifact in the CI

For packer you need to set the `PKR_VAR_enable_video_capture` var to `on` as packer is not as flexible as vagrant. The default for this var is `off` thus disabling the capture.

After enabling one of those vars, and only if the job fails, you will get the capture.webm file as an artifact in the CI.

