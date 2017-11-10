#!/usr/bin/env ruby

require 'fileutils'

# Basic script to add file extensions to older Mac files
USAGE = <<EOD
add_extensions.rb [--dry-run] [--ignore-existing] [PATH]

`--dry-run` prints the actions to be taken without performing them.
`--ignore-existing` ignores any extensions the files already have (e.g. if periods are used in the filename)
PATH is optional; defaults to current directory.
EOD

if ARGV.delete('--help') || ARGV.delete('-h')
  puts USAGE
  exit
end

dry_run = ARGV.delete('--dry-run')
ignore_existing = ARGV.delete('--ignore-existing')
path = ARGV.shift || Dir.pwd
path = File.expand_path(path)

puts "Scanning #{path} for files without extensions..."

known_extensions = {
  'AIFF' => '.aif',
  'Sd2f' => '.sd2',
}

Dir[File.join(path, '**/*')].each do |file|
  next unless File.file?(file)
  type = `GetFileInfo -t "#{file}"`.chomp.gsub('"', '')
  extension = known_extensions[type]
  if ignore_existing
    next if File.extname(file) == extension
  else
    next unless File.extname(file) == ''
  end
  if extension
    puts %(Renaming "#{file}" to "#{file}#{extension}".)
    unless dry_run
      FileUtils.mv(file, "#{file}#{extension}", verbose: true)
    end
  else
    $stderr.puts %(Don't know what extension to use for "#{type}" file type.)
  end
end

puts "Done."
