# server-sleeper

problem: i have a home server that i use to run a few things (media server, long-running downloads i don't want to leave a big computer on for, etc."

i want it to sleep when it's not doing work. this repo has 2 binaries that help me effectively scale it up and down.

1. Waker. This is a simple program that runs on my raspberry pi (low power cost, so i can leave it running forever) that wakes up the server whenever my TV turns on (since that's when I'll likely be using the media server)
2. Sleeper. This is another simple program that runs on the server itself. It sleeps if there hasn't been any relevant activity in 20 minutes, where relevant activity is either actively watching content or downloading something. 
