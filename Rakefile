# FreeBSD Ports
#
# Douglas Thrift
#
# Rakefile

require 'date'
require 'ostruct'
require 'pathname'
require 'set'
require 'tmpdir'

def in_portdir(&block)
  portdir = Pathname.new(Rake.original_dir)
  raise "#{portdir} is not a port directory" unless portdir.relative_path_from(@portsdir).to_s =~ %r{[^/]+/[^/]+}
  Dir.chdir(portdir) do
    block.call(portdir)
  end
end

def update_portstree_map(map = {})
  `poudriere ports -l -q`.each_line do |line|
    line.chomp!
    if line =~ /^([^ ]+) +([^ ]+) +([0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}) +([^ ]+)$/
      map[$1.to_sym] = OpenStruct.new(method: $2, timestamp: DateTime.parse($3), path: Pathname.new($4))
    else
      raise "unexpected line: #{line}"
    end
  end
  map
end

def with_portsdir(path, &block)
  # Set PORTSDIR around a command rather than prefixing it, so rake echoes the
  # command alone. Restored afterwards so it cannot leak into another task in
  # the same invocation (rake portlint diff).
  old = ENV['PORTSDIR']
  ENV['PORTSDIR'] = path.to_s
  block.call
ensure
  ENV['PORTSDIR'] = old
end

def clean_version(version)
  if version =~ /^(\d+\.\d+-RELEASE)(?:-p\d+)$/
    $1
  else
    version
  end
end

portstree_map = update_portstree_map
portsdir = @portsdir = Pathname.pwd
distdir = portsdir.join('distfiles')

task default: %i(update_githead)

desc 'Create or update the port checksum file (distinfo)'
task :makesum do
  in_portdir do
    sh "make makesum DEV_WARNING_WAIT=0 DISTDIR=#{distdir}"
  end
end

desc 'Regenerate the port patches in files from the edited sources'
task makepatch: :update_githead do
  in_portdir do
    # makepatch regenerates files/patch-* from .orig copies under WRKDIR. It is
    # not rake diff[patch], which produces the single diff a PR attaches; these
    # patches ship inside the port and the framework applies them at build time.
    #
    # make makepatch never fetches -- it regenerates patches from .orig copies
    # already under WRKDIR -- so unlike makesum this needs no DISTDIR. WRKDIR
    # and PATCHDIR both resolve inside this working copy whichever tree PORTSDIR
    # names. The one thing it takes from the tree is SCRIPTSDIR's
    # smart_makepatch.sh, which decides patch naming and whitespace handling --
    # so run the copy from the tree update_githead keeps current rather than
    # whatever /usr/ports happens to hold. That is why this task has a
    # prerequisite the make target does not.
    with_portsdir(githead_portstree.path) do
      sh "make makepatch"
    end
  end
end

# These are for checking a fetched distfile against the hash upstream publishes,
# not for distinfo -- CHECKSUM_ALGORITHMS is sha256 alone and makesum writes it.
# So the set tracks what projects publish: sha512 is the common one now, sha256
# and the older md5/sha1 still appear.
%w(md5 sha1 sha256 sha512).each do |sum|
  desc "Show the #{sum.upcase} sums of the distfiles"
  task sum do
    sh "mkdir -p #{distdir}" unless distdir.exist?
    files = distdir.glob('**/*').select {|dist| dist.file?}.map {|dist| dist.relative_path_from(portsdir)}
    sh "#{sum} #{files.join(' ')}" unless files.empty?
  end
end

desc 'Remove the ports distfiles'
task :distclean do
  in_portdir do
    sh "make distclean DISTDIR=#{distdir}"
  end
end

desc 'Verify the port using portlint'
task portlint: :update_githead do
  in_portdir do |portdir|
    # portlint reads MOVED, UIDs, GIDs and Mk/ out of PORTSDIR, which defaults
    # to /usr/ports -- a tree nothing here manages, and which was four days
    # behind when this was written. Point it at the githead tree this task
    # already pays a sudo to keep current, so the -C/-A decision below and the
    # lint itself consult one tree rather than two of different vintages.
    with_portsdir(githead_portstree.path) do
      if githead_portstree.path.join(portdir.relative_path_from(portsdir), 'Makefile').exist?
        sh "portlint -C"
      else
        sh "portlint -A"
      end
    end
  end
end

desc 'Check the port Makefile variable order and style using portclippy'
task :portclippy do
  in_portdir do
    sh "portclippy Makefile"
  end
end

desc 'Format the port Makefile in place using portfmt'
task :portfmt do
  in_portdir do
    sh "portfmt -i Makefile"
  end
end

def git_output(*command, env: {})
  output = IO.popen(env, ['git', *command], err: %i(child out), &:read)
  raise "#{['git', *command].join(' ')} failed with status #{$?.exitstatus}: #{output}" unless $?.success?
  output
end

desc 'Show the port differences against the ports tree, or write them as a patch'
task diff: :update_githead do |_, args|
  patch = args.extras.include?('patch')
  others = args.extras - ['patch']
  tree = others.find {|arg| portstree_map.key?(arg.to_sym)} || 'githead'
  commits = others - [tree]
  raise "unexpected arguments: #{commits.join(', ')}" if commits.size > 1
  commit = commits.first
  portstree = portstree_map[tree.to_sym] or raise "no such portstree: #{tree}"
  in_portdir do |portdir|
    rel_portdir = portdir.relative_path_from(portsdir)
    tree_portdir = portstree.path.join(rel_portdir)
    new_port = !tree_portdir.exist?

    # The patch carries the mail header of a commit here -- its author, date
    # and message -- so the diff below it has to be what that commit holds.
    # Anything not yet committed would make the two disagree, so the patch is
    # refused and the diff is shown without a header.
    clean = git_output('status', '--porcelain', '--', '.').empty?
    raise "#{rel_portdir} has uncommitted changes; commit them first" if patch && !clean
    if clean
      commit ||= git_output('log', '-1', '--format=%H', '--', '.').chomp
      raise "#{rel_portdir} has no commits" if patch && commit.empty?
    else
      $stderr.puts "#{rel_portdir} has uncommitted changes, so the diff has no mail header"
      commit = nil
    end

    # Color follows git's own color.diff and color.ui settings, as for a
    # terminal when this task's output is one; a written patch never has it.
    color = !patch && git_output('config', '--get-colorbool', 'color.diff', $stdout.tty?.to_s).chomp == 'true'

    # The files compared are those git sees here, tracked or untracked but not
    # ignored, so work directories and *.orig copies stay out; and everything
    # in the tree's copy, so that deletions still show. A tracked file deleted
    # but not yet committed is still listed, so only those on disk are kept.
    files = git_output('ls-files', '-z', '--cached', '--others', '--exclude-standard', '.').split("\0")
      .select {|file| File.exist?(file) || File.symlink?(file)}
    tree_files = new_port ? [] : tree_portdir.glob('**/*', File::FNM_DOTMATCH).select(&:file?).map {|file| file.relative_path_from(tree_portdir).to_s}

    # A new port also needs its SUBDIR line in the category Makefile, which is
    # not in this repo. The lines are in byte order, so it goes before the
    # first entry that sorts after it.
    category_makefile = rel_portdir.dirname.join('Makefile')
    if new_port
      name = rel_portdir.basename.to_s
      lines = portstree.path.join(category_makefile).readlines
      entry = "    SUBDIR += #{name}\n"
      unless lines.include?(entry)
        subdirs = lines.each_index.select {|i| lines[i].start_with?('    SUBDIR += ')}
        index = subdirs.find {|i| lines[i].split.last > name} || subdirs.last + 1
        lines.insert(index, entry)
      end
    end

    # Each side is added from where it sits on disk into its own index in a
    # throwaway repository, so git records file modes and symlinks itself and
    # nothing is copied into the ports tree. Diffing the two trees is what git
    # format-patch does, so --stat=72 --summary reproduces its diffstat
    # exactly, which git diff --no-index cannot: its paths are the absolute
    # ones, and the stat abbreviates them.
    diff = Dir.mktmpdir do |tmpdir|
      git_dir = "--git-dir=#{tmpdir}/repo"
      git_output('init', '-q', '--bare', "#{tmpdir}/repo")
      tree_side, port_side = [[portstree.path, tree_files], [portsdir, files]].map.with_index do |(root, paths), side|
        env = {'GIT_INDEX_FILE' => "#{tmpdir}/index#{side}"}
        paths = paths.map {|path| rel_portdir.join(path).to_s}
        paths << category_makefile.to_s if new_port && side == 0
        git_output('-C', root.to_s, '--literal-pathspecs', git_dir, "--work-tree=#{root}", 'add', '-f', '--', *paths, env: env) unless paths.empty?
        if new_port && side == 1
          makefile = Pathname.new(tmpdir).join('Makefile')
          makefile.write(lines.join)
          blob = git_output(git_dir, 'hash-object', '-w', makefile.to_s).chomp
          git_output(git_dir, 'update-index', '--add', '--cacheinfo', "100644,#{blob},#{category_makefile}", env: env)
        end
        git_output(git_dir, 'write-tree', env: env).chomp
      end
      git_output(git_dir, 'diff', "--color=#{color ? 'always' : 'never'}", '--stat=72', '--summary', '-p', '--src-prefix=a/', '--dst-prefix=b/', tree_side, port_side)
    end

    # The header and signature come from git format-patch itself, so the MIME
    # headers, the [PATCH] subject and the git version match what it would
    # have written.
    if commit && !commit.empty? && !diff.empty?
      email = git_output('format-patch', '-1', '--stdout', commit)
      header = email[/\A.*?^---\n/m] or raise "no --- line in git format-patch output for #{commit}"
      signature = email[/^-- \n.*\z/m] or raise "no signature in git format-patch output for #{commit}"
      diff = header + diff + signature
    end

    if patch
      # PKGNAME depends on Mk/ -- PYTHON_PKGNAMEPREFIX and friends come from
      # there -- so resolve it against the same tree being diffed against
      # rather than whatever PORTSDIR happens to default to.
      pkgname = with_portsdir(portstree.path) {`make -V PKGNAME`.chomp}
      file = portsdir.join("#{pkgname}.diff")
      file.write(diff)
      puts "wrote #{file}"
    else
      print diff
    end
  end
end

desc 'Test the port with poudriere'
task :testport, %i(jailname flavor) => %i(update_githead register_overlay) do |_, args|
  this_version = clean_version(`uname -r`.chomp)
  this_arch = `uname -m`.chomp
  this_jailname = nil
  `poudriere jail -l -q`.each_line do |line|
    line.chomp!
    if line =~ /^([^ ]+) +([^ ]+) +([^ ]+) +([^ ]+).+$/
      jailname, version, arch = $1, $2, $3
      # The creation method says nothing about whether a jail is the right one to
      # test in; version and architecture do. This used to require 'ftp', which
      # stopped matching anything when poudriere's default became 'http'.
      if clean_version(version) == this_version && arch == this_arch
        this_jailname = jailname
        break
      end
    else
      raise "unexpected line: #{line}"
    end
  end
  raise "no #{this_version} #{this_arch} jail found" if this_jailname.nil? && args.jailname.nil?
  args.with_defaults(jailname: this_jailname)
  in_portdir do |portdir|
    rel_portdir = portdir.relative_path_from(portsdir)
    # -b latest prefetches binary packages for the dependencies instead of
    # building them, matching the cron bulk builds and caronade's recipe.
    sh "sudo poudriere testport -b latest -j #{args.jailname} -o #{rel_portdir}#{args.flavor && "@#{args.flavor}"} -p githead -O overlay"
  end
end

{githead: 'git+https'}.each do |name, method|
  define_method("#{name}_portstree") do
    portstree_map[name]
  end

  desc "Create or update the #{name} portstree"
  task "update_#{name}" do |_, args|
    force = args.extras.find {|arg| arg == 'force'}
    portstree = portstree_map[name]
    if portstree
      raise "unexpected portstree method for #{name}: #{portstree[:method]} (expected: #{method})" if portstree[:method] != method
      if force || portstree.timestamp < Date.today
        sh "sudo poudriere ports -u -p #{name}"
      end
    else
      sh "sudo poudriere ports -c -m #{method} -p #{name}"
      update_portstree_map(portstree_map)
    end
  end
end

desc 'Register this working copy as the overlay portstree'
task :register_overlay do
  portstree = portstree_map[:overlay]
  if portstree
    raise "unexpected overlay portstree path: #{portstree.path} (expected: #{portsdir})" if portstree.path != portsdir
  else
    # -m null registers an existing directory without poudriere creating or
    # populating it, and -f none is required or the create errors out on the
    # directory already existing. poudriere testport -O then nullfs-mounts this
    # over the base tree, so the build reads this working copy directly and
    # nothing has to be copied into the root-owned githead tree.
    sh "sudo poudriere ports -c -m null -M #{portsdir} -f none -p overlay"
    update_portstree_map(portstree_map)
  end
end
