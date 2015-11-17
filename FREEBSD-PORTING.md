# Porting to FreeBSD
I'm trying to make docker work on freebsd

Major milestones for porting docker on FreeBSD are:

* make it compile (DONE)
* make it start as a daemon (DONE)
* load an image and create the container (aka working graphdriver) (DONE)
* run the container (DONE)
* working top\start\stop\kill (aka working execdriver) (DONE)
* working simple networking aka NAT on shared system interface (IN PROGRESS)
* working port forward (aka working networkdriver)
* working volumes and links
* working virtualized networking aka NAT on VINET 
* working limits
* major code cleanup and steps to push code to docker project (IN PROGRESS)

(See the bigger list below)

# Running
We dont have working docker image on freebsd, and cross-compile doesn't work wery well, so now we need to compile on FreeBSD directly

Prereqesites

    # pkg install go
    # pkg install git
    # pkg install sqlite3
    # pkg install bash
    # pkg install ca_root_nss # use this if pull command is not working

First we get the sources
    
    # git clone https://github.com/kvasdopil/docker 
    # cd docker
    # git checkout freebsd-compat
    
Now build the binary    

    # setenv AUTO_GOPATH 1
    # ./hack/make.sh binary 

This should build the docker executable ./bundles/latest/binary/docker. Now run the daemon:

    # zfs create -o mountpoint=/dk zroot/docker 
    # ./bundles/latest/binary/docker -d -e jail -s zfs -g /dk -D

After the daemon is started we can pull the image and start the container

    # ./bundles/latest/binary/docker pull lexaguskov/freebsd
    # ./bundles/latest/binary/docker run lexaguskov/freebsd echo hello world
   
Interactive mode works too

    # ./bundles/latest/binary/docker run -it lexaguskov/freebsd csh

# Networking

Docker provides each container an unique ip address on shared network interface

    # ./bundles/latest/binary/docker run -it lexaguskov/freebsd ifconfig lo1 

Now the docker can setup basic networking, but not nat, so we need to setup it manually

    # echo "nat on {you-external-interface} from 172.17.0.0/16 to any -> ({your-external-interface})" > /etc/pf.conf
    # pfctl -f /etc/pf.conf
    # pfctl -e

    # ./bundles/latest/binary/docker run -it lexaguskov/bsd ping ya.ru # this should work

# List of working commands and features

Commands:
* attach    - ok
* build
* commit
* cp        - ok
* create    - ok
* diff      - ok
* events    - ok
* exec      - crash
* export
* history   - ok
* images    - ok
* import
* info      - ok
* inspect   - ok
* kill
* load      - not working
* login
* logout
* logs      - ok
* pause     - not working (not supported on freebsd)
* port
* ps        - ok
* pull      - ok
* push
* rename    - ok
* restart   - ok
* rm        - ok
* rmi       - ok
* run       - ok
* save
* search    - ok
* start     - ok
* stats     - should not work (not implemented)
* stop      - ok
* tag
* top       - ok
* unpause   - not working (not supported on freebsd)
* version   - ok
* wait      - ok

Features:
* image loading         - ok
* container creation    - not working anymore
* container stop\start  - not working anymore
* build on FreeBSD 9.3  - ok
* build on FreeBSD 10.1 - ok
* shared networking     - partial support
* port forward          - ok
* volumes               - not working
* links                 - not working
* virtual netowrking    - not working
* limits                - not working

# Participating

If you wish to help, you can join IRC channel #freebsd-docker on freenode.net. 

Now we have following issues:
* not working "docker load"
* not working "docker stats"
* not working limits
* netlink functions from libcontainer are not working
* docker can't load (pull, import or commit) an image if not started from build path

Current progress is focused on networking, NAT and port forwarding.
