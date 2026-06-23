#!/bin/sh

# Debian's stock autopartition-crypto helper always turns the opened LUKS
# mapper into an LVM PV. Shoelaces regular encrypted storage keeps the normal
# partman crypto setup flow, then marks the mapper itself as root.

set -e

. /lib/partman/lib/base.sh
. /lib/partman/lib/recipes.sh
. /lib/partman/lib/auto-shared.sh
. /lib/partman/lib/crypto-base.sh

dev="$1"

clean_method

cd "$dev"
[ -f size ] || exit 1
size=$(cat size)
target="$(humandev "$(cat device)") - $(cat model): $(longint2human "$size")"
free_size=$(convert_to_megabytes "$size")

choose_recipe default "$target" "$free_size" || exit $?
auto_init_disks "$dev" || exit $?
get_last_free_partition_infos "$dev"
perform_recipe "$dev" "$free_space" "$recipe" || exit $?

found=no
for devdir in "$DEVICES"/*; do
	[ -d "$devdir" ] || continue
	cd "$devdir"
	partitions=
	open_dialog PARTITIONS
	while { read_line num id size type fs path name; [ "$id" ]; }; do
		[ "$fs" != free ] || continue
		partitions="$partitions $id,$path"
	done
	close_dialog

	for part in $partitions; do
		id=${part%,*}
		[ -f "$id/method" ] || continue
		[ "$(cat "$id/method")" = crypto ] || continue

		echo dm-crypt > "$id/crypto_type"
		crypto_prepare_method "$devdir/$id" dm-crypt || exit 1

		if [ -f "$id/options/cipher" ]; then
			cipher="$(cat "$id/options/cipher")"
			echo "${cipher%%-*}" > "$id/cipher"
			echo "${cipher#*-}" > "$id/ivalgorithm"
		fi
		if [ -f "$id/options/keysize" ]; then
			keysize="$(cat "$id/options/keysize")"
			# partman-crypto doubles keysize for XTS before invoking
			# cryptsetup. Shoelaces passes the cryptsetup key size, so
			# convert it back to partman's internal per-half key size.
			case "$(cat "$id/ivalgorithm" 2>/dev/null)" in
				xts-*)
					case "$keysize" in
						''|*[!0-9]*) ;;
						*) keysize="$((keysize / 2))" ;;
					esac
					;;
			esac
			echo "$keysize" > "$id/keysize"
		fi
		[ ! -f "$id/options/hash" ] || cat "$id/options/hash" > "$id/keyhash"

		# The filesystem and mountpoint belong on the opened mapper, not the
		# backing crypto partition.
		rm -f "$id/format" "$id/use_filesystem" "$id/filesystem" "$id/mountpoint"
		# d-i does not have wipefs. Remove old filesystem/LUKS signatures with
		# dd so cryptsetup does not reject a reused partition.
		dd if=/dev/zero of="$path" bs=1M count=16 conv=fsync || true
		sectors="$(blockdev --getsz "$path" 2>/dev/null || echo 0)"
		if [ "$sectors" -gt 32768 ]; then
			seek="$((sectors - 32768))"
			dd if=/dev/zero of="$path" bs=512 seek="$seek" count=32768 conv=fsync || true
		fi
		touch "$id/skip_erase"
		found=yes
	done
done

[ "$found" = yes ] || exit 1

crypto_check_setup || exit 1
crypto_setup no || exit 1

for cryptdev in "$DEVICES"/*; do
	[ -d "$cryptdev" ] || continue
	[ -f "$cryptdev/crypt_realdev" ] || continue
	cd "$cryptdev"
	partitions=
	open_dialog PARTITIONS
	while { read_line num id size type fs path name; [ "$id" ]; }; do
		[ "$fs" != free ] || continue
		partitions="$partitions $id"
	done
	close_dialog

	for id in $partitions; do
		mkdir -p "$id"
		echo format > "$id/method"
		: > "$id/format"
		: > "$id/use_filesystem"
		echo ext4 > "$id/filesystem"
		echo / > "$id/mountpoint"
	done
done

update_all
menudir_default_choice /lib/partman/choose_partition finish finish || true
