all: 
	@./build.sh
clean:
	@rm -f forensic
install: all
	@cp triangle /usr/local/bin
uninstall: 
	@rm -f /usr/local/bin/forensic
package:
	@NOCOPY=1 ./build.sh package