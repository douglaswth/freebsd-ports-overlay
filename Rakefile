# FreeBSD Ports
#
# Douglas Thrift
#
# Rakefile

require 'date'
require 'ostruct'
require 'pathname'
require 'pry'
require 'set'

Color = Pry::Helpers::Text

def in_portdir(&block)
  Dir.chdir(Rake.original_dir) do |portdir|
    block.call(Pathname.new(portdir))
  end
end

def in_portstree(portstree, &block)
  portstree_zfs = `zfs get -H -o name mountpoint #{portstree.path}`.chomp
  sh "sudo zfs snapshot #{portstree_zfs}@clean"
  Dir.chdir(portstree.path) do
    block.call(portstree.path)
  end
ensure
  sh "sudo zfs rollback #{portstree_zfs}@clean"
  sh "sudo zfs destroy #{portstree_zfs}@clean"
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

portstree_map = update_portstree_map
portsdir = Pathname.pwd
distdir = portsdir.join('distfiles')

task default: %i(update_githead update_svnhead)

desc 'Create or update the port checksum file (distinfo)'
task :makesum do
  in_portdir do
    sh "make makesum DEV_WARNING_WAIT=0 DISTDIR=#{distdir}"
  end
end

%w(md5 sha1 sha256).each do |sum|
  desc "Show the #{sum.upcase} sums of the distfiles"
  task sum do
    sh "mkdir -p #{distdir}" unless distdir.exist?
    sh "#{sum} #{distdir.join('*')}"
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
    if githead_portstree.path.join(portdir.relative_path_from(portsdir), 'Makefile').exist?
      sh "portlint -C"
    else
      sh "portlint -A"
    end
  end
end

desc 'Show the port differences'
task diff: :update_svnhead do |_, args|
  patch = args.extras.find {|arg| arg == 'patch'}
  in_portdir do |portdir|
    new_files = Set.new(`git ls-files`.each_line.map{|line| Pathname.new(line.chomp)})
    in_portstree(svnhead_portstree) do |path|
      rel_portdir = portdir.relative_path_from(portsdir)
      svnhead_portdir = path.join(rel_portdir)
      if svnhead_portdir.exist?
        old_files = Set.new(`svn ls -R #{svnhead_portdir}`.each_line.reject {|line| line =~ %r{/$}}.map {|line| Pathname.new(line.chomp)})
      else
        old_files = Set.new
      end
      sh "sudo mkdir -p #{svnhead_portdir}"
      Dir.chdir(svnhead_portdir) do
        new_files.each do |file|
          portdir_file = portdir.join(file)
          sh "sudo mkdir -p #{file.dirname}" unless file.dirname.exist?
          sh "sudo cp #{portdir_file} #{file}"
        end
        (old_files - new_files).each do |file|
          sh "sudo svn add --parents #{file}"
        end
        (new_files - old_files).each do |file|
          sh "sudo svn remove #{file}"
        end
        unless old_files.empty?
          if patch
            sh "svn diff > #{portdir.dirname.join("#{`make -V PKGNAME`.chomp}.diff")}"
          else
            sh "svn diff"
          end
        end
      end
      if old_files.empty?
        if patch
          sh "svn diff #{rel_portdir} > #{portsdir.join("#{portdir.basename}.diff")}"
        else
          sh "svn diff #{rel_portdir}"
        end
      end
    end
  end
end

{githead: 'git+https', svnhead: 'svn+https'}.each do |name, method|
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
