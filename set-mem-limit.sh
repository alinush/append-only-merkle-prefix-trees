mb=${1:-'200'}

echo "WARNING: You must source this script for it to have any effect:"
echo " $ . set-mem-limit.sh"
echo "Check for success by running 'ulimit -v' in your shell."
echo " $ ulimit -v"
echo
echo "Setting memory limit to ${mb}MB ..."

mb=$((mb * 1024))

echo "(or to ${mb} bytes)"

ulimit -Sv $mb
