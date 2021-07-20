#!/usr/bin/env bash

version=$1
steps=0
steps_complete=0

# Check if the working directory is in the state we expect it to be in
sanity_checks () {
	is_git=$(git rev-parse --is-inside-work-tree)
	if [ "$is_git" != "true" ]; then
		exit 1
	fi

	current_branch=$(git rev-parse --abbrev-ref HEAD)
	if [ "$current_branch" != "main" ]; then
		printf "\e[1mWarning:\e[0m You are not on the main branch.\n"
		read -n 1 -p "Do you really want to continue? ([y]es, [N]o) " response
		printf "\n"
		if [ "$response" != "y" ]; then
			exit 1
		fi
	fi

    echo "Updating $current_branch to avoid conflicts..."
	if [ -n "$(git status --untracked-files=no --porcelain)" ]; then
		printf "\e[1mWarning:\e[0m The working directory is not clean.\nYou have the following unstaged or uncommitted changes:\n"
		git status --untracked-files=no -s
		read -n 1 -p "Do you really want to continue? ([y]es, [N]o) " response
		printf "\n"
		if [ "$response" != "y" ]; then
			exit 1
		fi
	else
		git pull
	fi
}

# Check if the input parameters (version) were provided
test_parameters () {
	# Get the latest tag and increment the version by 1
	latest_tag=$(git describe --abbrev=0 --match "$v*")
	if [ "$latest_tag" = "" ]; then
		echo "Error: There seem to be no valid tags."
		exit 1
	fi

	version=$(echo $latest_tag | sed 's/^v\(.*\)/\1/')
    new_version=$(echo "$(($version + 1))")

	# Get the version
	if [ -z "$1" ]; then
		read -p "Specify a version (Default: $new_version): " version
		if [ -z "$version" ]; then
			version="$(echo "$new_version")"
		fi
	else
		version=$1
	fi

	if [ "$(git tag | grep -c $version\$)" = "1" ]; then
		printf "\e[1mWarning:\e[0m The version you specified ('$version') exists as a git tag. "
		read -n 1 -p "Do you really want to release again? ([y]es, [N]o) " response
		printf "\n"
		if [ "$response" != "y" ]; then
			exit 1
		fi
	fi

	echo "Version: $version"
}

# Print the step info
step () {
	printf "\n\n \e[1mStep $steps: $1\e[0m\n ==================\n"
}

# Ask whether the step should be executed
run_command () {
	let steps++
	read -n 1 -p " → Do it? ([y]es, [N]o, [s]kip) " response
	printf "\n"
	if [ "$response" = "y" ]; then
		eval $1 && eval $2 && eval $3
		printf "\n ✓ Done."
		let steps_complete++
	elif [ "$response" = "s" ]; then
		printf "\n Step $(( $steps - 1 )) skipped."
		return
	else
		read -n 1 -p " Step $(( $steps - 1 )) aborted. Do you really want to quit? ([y]es, [N]o) " abort
		if [ "$abort" = "y" ]; then
			printf "\n Aborted. (Steps complete: $steps_complete)\n"
			exit 0
		else
			printf "\n Step $(( $steps - 1 )) aborted. Continuing...\n"
			return
		fi
	fi
}

# Playbook for all release steps
run_steps () {
    step "Check out a new branch for the release \e[0m(git checkout -b release-version-$version)"
    run_command "git checkout -b release-version-$version"

    step "Create docs dir and populate it"
    run_command "mkdir -p docs/news/$version" "mv docs/news/unreleased/* docs/news/$version"

    step "Generate template for new release \e[0m(make release)"
    run_command "make release"

    step "Edit the release notes \e[0m(vi NEWS.md)"
    run_command "$(git config --default "${EDITOR:-vi}" --global core.editor) NEWS.md"

    step "Update the version number in the spec file \e[0m(vi osbuild-composer.spec)"
    run_command "sed -i -E 's/(Version:\\s+)[0-9]+/\1$version/' osbuild-composer.spec" "git diff osbuild-composer.spec"

    step "Commit the updated spec, the NEWS.md and the docs e[0m(git commit -a)"
    run_command "git add osbuild-composer.spec NEWS.md docs/news/unreleased docs/news/$version" "git commit -s -m $version -m 'Release osbuild-composer $version'"
}

### Main loop

main () {
	sanity_checks
	test_parameters $version
	run_steps

	printf "\n\nCongrats, you completed $steps_complete of $steps steps of doing a release."
	printf "\nAs the final step, push the branch into your fork and open a PR against the main branch. The PR now needs to be approved and merged.\n"
}

main
