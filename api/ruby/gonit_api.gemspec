$:.unshift(File.join(File.dirname(__FILE__), 'lib'))

require 'gonit_api/version'

Gem::Specification.new do |s|
  s.author = "Doug MacEachern"
  s.name = 'gonit_api'
  s.version = GonitApi::VERSION
  s.summary = 'Gonit API client'
  s.description = s.summary
  s.homepage = 'https://github.com/cloudfoundry/gonit'

  s.add_development_dependency "rspec"
  s.add_development_dependency "ci_reporter"

  s.files = `git ls-files`.split("\n")
  end
