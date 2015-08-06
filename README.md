# secret-agent-goph3r

I was at Gophercon 2015 in Denver a few weeks ago and tried my hand
at a programming challenge hosted by CoreOS. Unfortunately I didn't
end up finishing it and was sad that they no longer had it up after the
conference. I did what any rational person would do and tried recreating
their challenge.

You play the game by opening up a raw tcp connection and joining a
channel with a few friends.  The point of the game is to send a third
party sensitive files without getting caught by security.  Behind the
core of game is a fairly complex and practical math problem, the multiple
knapsack problem.  I wasn't able to implement an algorithm for solving
the MKP yet so that I could generate random problems and calculate a
top score, but that is in the pipeline of things to do. For now, I have
taken a sample problem from a textbook that is quite easy to solve and
no top score is listed. I also want to create a seed of names from which
to generate problem data.  Of course, you could in theory solve this
challenge without any programming at all, but that wouldn't be much fun
now, right?

