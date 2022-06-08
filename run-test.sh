set -e

if [ $# -lt 1 ]; then
    echo "Usage: $0 <filename-prefix> [<size-1> <size-2> ...]"
    exit 1
fi

make

filePrefix=$1
shift || :

for i in `seq 1 10`; do
    echo
    echo "Running with seed $i..."
    echo
    ./hashperiments "$i" "$filePrefix-seed-$i.csv" $@ 2>&1 | tee $filePrefix-seed-${i}.log
done
