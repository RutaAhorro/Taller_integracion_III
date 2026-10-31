## CHAPTER 13

## Software Security

## ACRONYMS

| CC   | Common Criteria               |
|------|-------------------------------|
| SDLC | Secure Development Life Cycle |

## INTRODUCTION

Security has become a significant issue in software development because of potential rity faults and loopholes can be and often are misuse and increasing malicious activity targeting computer systems. In addition to the usual correctness and reliability concerns, 1.1. Software Security [10] software developers must pay attention to the security of the software they develop. Security is a product quality characteristic Secure software development builds securepresenting the degree to which a product or rity by following a set of established and/ system protects information and data so that or recommended rules and practices. Secure persons or other products or systems have data software maintenance complements secure access appropriate to their types and levels of software development by ensuring that no authorization [10]. (For more information security problems are introduced during softabout product quality, refer to the Software ware maintenance and that identified vulnerQuality KA.) abilities, which are errors that attackers can exploit, can be handled during the software 1.2. Information Security [11] life cycle. Security vulnerabilities are not only introduced at the development, but also by third party components such as libraries, COTS, or OS.

security into software than to patch it in after the software is developed. To design security into software, one must consider every development life cycle stage. Secure software development involves software requirements security, software design security, software construction security and software testing security. In addition, security must be considered during software maintenance, as secuintroduced during maintenance.

Information security preserves confidentiality, integrity and availability of information. Other properties, such as authenticity, accountability, non-repudiation and reliability can also be involved [11]. Confidentiality is BREAKDOWN OFTOPICS FOR the property of ensuring that information is SOFTWARE SECURITY not disclosed to unauthorized individuals, entities or processes. Integrity is the property Breakdown of topics for the Software Security of accuracy and completeness. Availability KA is shown in Figure 13.1. is the property of being accessible and usable on demand by an authorized entity. 1. Software Security Fundamentals [37, 9] Software engineers should define the security properties of their software and maintain them throughout the software development

A generally accepted belief about software sec  es e   e e e t le.

Figure 13.1. Breakdown of Topics for the Software Security KA

<!-- image -->

<!-- formula-not-decoded -->

## 1.3. Cybersecurity

Cybersecurity is safeguarding of people, at a tolerable level.

society, organizations and nations from cyber Many organizations practice security engirisks. Safeguarding means to keep cyber risk neering in the development of computer programs, including operating systems, functions Generally, cybersecurity addresses secu- that manage and enforce security, packaged rity issues in cyberspace, including the software products, middleware, and applicafollowing: tions. Therefore, a diverse array of individuals must know how to apply appropriate methods · Social engineering attacks and practices, including product developers, · Hacking service providers, system integrators, system · Malicious software (malware) administrators and even security specialists. · Other potentially unwanted software [12] Systems Security Engineering — Capability Maturity Model (SSE-CMM), which helps Software engineers should consider the measure the process capability of an organimitigation of such threats as part of software zation that performs risk assessments [14], can development. be an important tool.

## 2. Security Management and Organization 2.2. Information Security Management System [1*, c7][13]

## [15]

Security governance and management are ISO/IEC 27001:2022 specifies the requiremost effective when they are systematic; in ments for establishing, implementing, other words, when they are woven into the e maintaining and continually improving an culture and fabric of organizational behaviors information security management system and actions. Project managers need to elevate (ISMS) within the organizational context -etn  an o  s n oa] - o-s  o n oron nical concern to an enterprise issue [1]. aging the technology-related security of an organization. This includes documenting risks Recently, DevSecOps (meaning the integraand taking measures to address them, aiming tion of development, security and operations) to protect the organization's data and prevent has emerged. Beyond SDLC, DevSecOps security breaches [15]. Organizations should includes an approach to culture, automation use it to continually conduct risk assessand platform design to make the software life ments to identify security risks and vulnera- cycle as Agile and responsible as Agile develbilities and implement protective measures by opment and continuous integration (CI). deploying an IT team to monitor these risks. An ISMS can thus also raise new or changed 3.2. Common Criteria for Information existing software security requirements. In Technology Security Evaluation addition, software security requirements are [3*, c22, c25][34][35] derived from laws, regulations and obligations for compliance.

<!-- formula-not-decoded -->

Agile teams need to understand and adopt rity needs or standards conformity. ISO/ security practices and take more responsibility IEC 15408:2022, named Common Criteria for their systems' security. Security profes- (CC) for Information Technology Security sionals must learn to accept change, work faster Evaluation, is useful as a guide for developing, and more iteratively, and think about security evaluating and/or procuring IT products with risks and how to manage risks in incremental terms. Finally, and most important, secuCC addresses the protection of assets from rity needs to become an enabler instead of a unauthorized disclosure, modification or loss blocker. The keys to a successful Agile security of use. The categories of protection relating program are the involvement of the security to these three types of security failure are team and developers, enablement, automation, commonly called confidentiality, integrity and and agility to keep up with Agile teams [4]. availability, respectively.

Security evaluation establishes confidence in the security functionality of IT products and the assurance measures applied to them. The [4,c15,c16]evaluation results may help consumers determine whether IT products meet their secusecurity functionality [34].

## 3. Software Security Engineering and Processes

## 4. Security Engineering for Software Systems [1*,c1,c3][3*,c1,c3]

- 3.1. Security Engineering and Secure

Development Life Cycle (SDLC)

[1*,c1][16][36]

Security requirements engineering includes Software is only as secure as its development elicitation, specification, and prioritization. It process. Security must be built into software considers threats, as illustrated by misuse and engineering to ensure software security. The abuse cases, threat actors, security risk assessSDLC concept is one trend that aims to do ments, selection and application of specithis. SDLC uses a classical spiral model that fication methods, prioritization methods, views security holistically from the perspective inspections, and revisions. Selection of lifeof the software life cycle and ensures that secucycle models may impact the order of activities, rity is inherent in software design and developand software product revision implies a need ment, not an afterthought later in production. to revisit security requirements. Traceability The SDLC process is claimed to reduce soft- of security requirements throughout the ware maintenance costs and increase software development process is important, and secureliability against security-related faults. rity teams may include specialists in security

<!-- formula-not-decoded -->

requirements. Numerous methods and tools The term software construction security can exist in support of security requirements engineering.

unauthorized disclosure, creation, change, deletion or denial of access to information and other resources. It also concerns how to tolerate security-related attacks or violations by limiting damage, continuing failing and recovering securely. Access control is a fundamental concept of security. The answer could be different for different Most controls build on cryptographic algocombinations of ISAs and compilers. Because rithms and cryptographic material like keys. of this lack of understanding, software conIt is important to carefully select these and struction security — in its current state — how cryptographic material is created, dismostly refers to the second aspect mentioned tributed and managed. above: the coding of security into software. Software design security deals with the Coding of security into the software can be

mean different things to different people. It can mean the way a specific function is coded so that the code itself is secure, or it can 4.2. Security Design mean the coding of security into software. [1*,c4][2,c5][3*,c20,c31][17,40]Unfortunately, most people entangle the two meanings without distinction. One reason Security design concerns how to prevent for such confusion is that it is unclear how to ensure a specific coding is secure. For example, in the C programming language, the expressions "i&lt;&lt;1" (shift the binary representation of i's value to the left by one bit) and "2*i" (multiply the value of variable i by constant 2) mean service, speeding repair and recovery, and the same thing semantically, but do they have the same security ramifications?

design of software modules that fit together achieved by following recommended rules. A t:e s s e s ees se se   ae: the security requirements. To meet security requirements, developers conduct threat · Structure the process so that all secmodeling, illustrating how a system is being tions requiring extra privileges are modattacked to specify a security design for the ules. The modules should be as small as mitigation. This step clarifies the details of possible and perform only the tasks that security considerations and develops the sperequire those privileges. cific steps for implementation. Factors con· Ensure that any assumptions in the prosidered may include frameworks and access gram are validated. If this is not possible, modes that set up the overall security mondocument them for the installers and itoring/enforcement strategies, as well as the maintainers so they know the assumpindividual policy enforcement mechanisms. tions attackers will try to invalidate.

<!-- formula-not-decoded -->

A security pattern describes a particular recurring security problem that arises in a specific context and presents a well-proven generic solution [21].

<!-- formula-not-decoded -->

- Ensure that the program does not share objects in memory with any other program.
- Check every function's error status. Do not recover unless neither the error's cause nor its effects affect any security considerations. The program should restore the state of the software to the state it had before the process began and then terminate.

Software construction security concerns how Although there are no bulletproof ways to to write programming code for specific sit- achieve secure software development, some uations to address security considerations. general guidelines exist that can be helpful.

These guidelines span every phase of the software development life cycle. The Computer Emergency Response Team (CERT/CC) publishes reputable guidelines [22], and the following are its top 10 software security practices:

1. Validate input.
2. Heed compiler warnings.
3. Architect and design for security policies.
4. Keep it simple.
5. Default deny.
6. Adhere to the principle of least privilege.
7. Sanitize data sent to other software.
8. Practice defense in depth.
9. Use effective quality assurance techniques.
10. Adopt a software construction security standard.

<!-- formula-not-decoded -->

Security testing ensures that the implemented software meets the security requirements. It also verifies that the software implementation contains none of the known vulnerabilities. Whereas general software testing methods can handle the former, the latter requires security-specific testing methods. (For more information about testing, refer to the Software Testing KA.)

There are two general approaches to security-specific testing. The first approach includes detecting vulnerabilities through static analysis, which can be conducted on the source code or compiled binaries. A static analysis on the source code can be used to detect programming language or implementation-specific vulnerabilities, while static analysis on compiled binaries can be used to detect vulnerabilities that are not apparent in the source code due to compiler optimizations or hidden in the compiled thirdparty components. Static analysis can be automated using tools, however while automation can play a significant role, the expertise of security professionals are required to properly operate and configure the tools, and verify the results.

The other approach to detect vulnerabilities is through dynamic testing, typically using techniques such as vulnerability assessment or penetration testing (also known as the ethical hacking test), to detect vulnerabilities in software behavior. Like static analysis, there are tools that can automate dynamic testing, such as web application scanners and fuzzing tools. Security experts skilled in the application domain should be engaged to perform these tests, and such tests should always be conducted within legal boundaries and with proper authorization. The latter aspects are crucial to differentiate such tests from illegal hacking activities.

<!-- formula-not-decoded -->

[1*,c5][3*,c24][28,29, 30]

Using sound coding practices can help substantially reduce software defects commonly introduced during implementation [1]. Such common security defects are categorized and shared with databases: the Common Vulnerabilities and Exposures (CVE) [28], Common Weakness Enumeration (CWE) [29], and Common Attack Pattern Enumeration and Classification (CAPEC) [30]; Common Vulnerability Scoring System (CVSS) [41] expresses characteristics and severity of software vulnerabilities. Programmers can refer to these databases for security implementation, and some tools are available to check common vulnerabilities in code. Security maintenance encompasses the task to mitigate effects of vulnerabilities in a system and third party components which the system uses. The task often comes with a vulnerability disclosure process that allows to report the identification of vulnerabilities.

## 5. Software SecurityTools

<!-- formula-not-decoded -->

<!-- formula-not-decoded -->

Security vulnerability checking tools, such as source code analyzers and binary analysis tools, can be used to identify potential security vulnerabilities and issues. Source One important difference with cloud envicode analyzers scrutinize code to detect ronments is that physical assets and protecsecurity vulnerabilities, such as injection are generally not a concern. Developers tion flaws, buffer overflows, and insecure can outsource asset tags, anti-tailgating, slablibrary use. They are useful at finding vul- to-slab barriers, placement of data center winnerabilities that can be identified through dows, cameras, and other physical security code patterns and logical flaws. Binary and physical asset tracking controls [31]. analysis tools, on the other hand, examine compiled code, including third-party 6.2. Security for IoT Software [32,33] libraries, for vulnerabilities that might not be apparent in the source code or that arise from the compilation process. While these tools significantly aid in detecting vulnerabilities, they cannot find all vulnerabilities. For example, they might not capture vulnerabilities that manifest in hard-to-produce software states or that crop up in unusual circumstances [1].

As part of today's internet of things (IoT), systems are interconnected with many other devices, especially back-end systems suffering from all the well-known security flaws inherent in today's business IT. Attackers gaining access to business IT platforms, for instance, by exploiting browser vulnerabilities, will likely also gain access to weakly protected IoT industrial devices. This can 5.2. Penetration Testing Tools [2,c4] cause severe damage, including safety incidents. Hence, the introduction of a massive Penetration testing tools can be used to evalnumber of end points from the consumer or uate a system's security in its operational industrial environment creates fertile ground environment. These tools perform controlled for the exploitation of weak links. Hardening attacks on the system to uncover vulnerabilithese end points, securing device-to-deties and security weaknesses, using techniques vice communications, and ensuring device such as fuzzing [2], where malformed, maliand information credibility in what until cious, or random data is submitted to the sysnow have been closed, homogeneous system's various entry points to detect faults. The tems present new challenges. Comprehensive use of penetration testing tools to expose vulrisk and threat analysis methods, as well as nerabilities provide insights into how an actual management tools for IoT platforms, are attacker could exploit the system. required [33].

## 6. Domain-Specific Software Security

<!-- formula-not-decoded -->

[31*,c1-c3]] Although machine learning techniques are widely used in many systems, machine learning presents a specific vulnerability. Attackers can change the decisions of machine learning models. There are two kinds of attacks: model poisoning, which attacks training data, and evasion, which attacks inputs to trained models [39].

Cloud infrastructure and services are often inexpensive and easy to provision, which can quickly lead to having many assets strewn all over the world and forgotten. These forgotten assets are like a ticking time bomb, waiting to explode into a security incident [31].

<!-- formula-not-decoded -->

## MATRIX OFTOPICS VS. REFERENCE MATERIAL

| Topic                                                               | Allen et al. 2008 [1*]   | Bishop 2019 [3*]   | Dotson 2019 [31*]   |
|---------------------------------------------------------------------|--------------------------|--------------------|---------------------|
| 1. Software Security Fundamentals                                   |                          |                    |                     |
| 1.1. Software Security                                              |                          |                    |                     |
| 1.2. Information Security                                           |                          |                    |                     |
| 1.3. Cybersecurity                                                  |                          | c23                |                     |
| 2. Security Management and Organization                             | c7                       |                    |                     |
| 2.1. Capability Maturity Model                                      |                          | c22                |                     |
| 2.2. Information Security Management System                         |                          |                    |                     |
| 2.3. Agile Practice for Software Security                           |                          |                    |                     |
| 3. Software Security Engineering and Processes                      |                          |                    |                     |
| 3.1. Security Engineering and Secure Development Life Cycle         | c1                       |                    |                     |
| 3.2. Common Criteria for Information Technology Security Evaluation |                          | c22, c25           |                     |
| 4. Security Engineering for Software Systems                        | c1, c3                   | c1, c3             |                     |
| 4.1. Security Requirements                                          | c3                       | c20,c31            |                     |
| 4.2. Security Design                                                | c4                       | c20,c31            |                     |
| 4.3. Security Patterns                                              | c4                       |                    |                     |
| 4.4. Construction for Security                                      | c5                       | c20, c31           |                     |
| 4.5. Security Testing                                               | c5                       | c24,c31            |                     |
| 4.6. Vulnerability Management                                       |                          | c24                |                     |
| 5. Software Security Tools                                          |                          |                    |                     |
| 5.1. Security Vulnerability Checking Tools                          | c6                       |                    |                     |
| 5.2. Penetration Testing Tools                                      | c4                       | c31                |                     |
| 6. Domain-Specific Software Security                                |                          |                    |                     |
| 6.1. Security for Container and Cloud                               |                          |                    | c1-c3               |
| 6.2. Security for IoT Software                                      |                          |                    |                     |
| 6.3. Security for Machine Learning-Based Application                |                          |                    |                     |

## FURTHER READINGS

- J. Viega, Building Secure Software: How to Avoid Security Problems the Right Way, Addison-Wesley, 2011.

This book introduces the definition of Software Security and the activities to develop and maintain secure software. It includes not only the software development process but also the related activities such as auditing and the monitoring of service.

L. Kohnfelder, Designing Secure Software: A Guide for Developers, No Starch Press, 2021.

This book describes security activities in the software design and implementation phases, including secure programming and web security. It also introduces best practices for secure software development.

C.W. Axelrod, Engineering Safe and Secure Software Systems, Artech House Publishers, 2012.

This book describes engineering activities to make software systems safe and secure from a risk management viewpoint. It introduces risk assessment and mitigation methods for security and safety.

## REFERENCES

- [1*] J.H. Allen, S.J. Barnum, R.J. Ellison, G. McGraw, and N.R. Mead, Software Security Engineering: A Guide for Project Managers, Addison-Wesley Professional, 2008.
- [2] G. McGraw, Software Security: Building Security In, Addison-Wesley Professional, 2006.
- [3] M. Bishop, Computer Security, 2nd Edition, Addison-Wesley Professional, 2019.
- [4] L. Bell, M. Brunton-Spall, R. Smith, and J. Bird, Agile Application Security, O'Reilly, 2017.
- [5] T. Hsiang-Chih Hsu, Hands-On Security in DevOps: Ensure continuous security, deployment, and delivery with DevSecOps, Packt Publishing, 2018.

Automation and Testing: Tools and techniques for automated security scanning and testing in DevSecOps, Packt Publishing, 2019.

- [7] G. Wilson, DevSecOps: A leader's guide to producing secure software without compromising flow, feedback and continuous improvement, Rethink Press, 2020.
- [8] L. Rice, Container Security: Fundamental Technology Concepts That Protect Containerized Applications, O'Reilly &amp; Associates Inc., 2020.
- [9] ISO/IEC/JTC1 SC27 Standards: Trustworthiness, Cryptography, Data security, Cryptography, Security evaluation and testing, Security control, Identity management and privacy technologies.
- [10] ISO/IEC 25010:2023 Systems and software engineering — Systems and software Quality Requirements and Evaluation (SQuaRE) — Product quality model.
- [11] ISO/IEC 27000:2018 Information technology — Security techniques — Information security management systems — Overview and vocabulary.
- [12] ISO/IEC 27032:2012 Information technology — Security techniques — Guidelines for cybersecurity.
- [13] ISO/IEC 19770-1:2017 Information technology — IT asset management — Part 1: IT asset management systems — Requirements.
- [14] ISO/IEC 21827:2008 Information technology — Security techniques — Systems Security Engineering — Capability Maturity Model (SSE-CMM).
- [15] ISO/IEC 27001:2022 Information
- [6] T. Hsiang-Chih Hsu, Practical Security

- security, cybersecurity and privacy pro- [26] K. Scarfone, M. Souppaya, A. Cody, tection — Information security manand A. Orebaugh, Technical Guide agement systems — Requirements. to Information Security Testing and Assessment, NIST SP800-115, 2008.
- [16] M. Howard and S. Lipner, The Security Development Lifecycle, Microsoft Press, 2006.
- [17] F. Swiderski and W. Snyder, Threat Modeling: Design for Security, Wiley, 2014.
- [18] D. Firesmith, "Security use cases," Journal of Object Technology, Vol. 2, No. 1, pp. 53-64, 2003.
- [27] PCI Security Standards Council, PCI DSS: Payment Card Industry Data Security Standard, Version 3.2, 2017.
- [28]MITRE, "Common Vulnerabilities and Exposures (CVE)," https://www.cve.org/.
- [29] MITRE, "Common Weakness Enumeration (CWE)," https://cwe. mitre.org/.
- [19] E. Fernandez-Buglioni, Security Patterns in Practice: Designing Secure Architectures Using Software Patterns, Wiley, 2013.
- [30] MITRE, "Common Attack Pattern Enumeration and Classification (CAPEC)," https://capec.mitre.org/.
- [20] C. Nagappan, R. Lai, and R. Steel, Core Security Patterns: Best Practices and Strategies for J2EE, Web Services, and Identity Management, Prentice Hall, 2005.
- [21] M. Schumacher, E. FernandezBuglioni, D. Hybertson, F. Buschmann, and P. Sommerlad, Security Patterns: Integrating Security and Systems Engineering, Wiley, 2006.
- [22] R.C. Seacord, The CERT C Secure Coding Standard, Addison-Wesley Professional, 2008.
- [23] R.C. Seacord, Secure Coding in C and C++, Addison-Wesley Professional, 2013.
- [24] D. Long, F. Mohindra, D. Seacord, R.C. Sutherland, and D.F. Svoboda, The CERT Oracle Secure Coding Standard for Java, 2011.
- [25] J. Erickson, Hacking: The Art of Exploitation, 2nd Edition, No Starch Press, 2008.
- [31*] C. Dotson, Practical Cloud Security, O'Reilly, 2019.
- [32]"Internet of Things Security Best Practices," IEEE, 2017, https:// standards.ieee.org/wp-content/uploads /import/documents/other/whitepaper -internet-of-things-2017-dh-v1.pdf.
- [33]"IoT 2020: Smart and secure IoT platform," IEC, 2016, https://www.iec.ch /basecamp/iot-2020-smart-and-secure -iot-platform.
- [34]ISO/IEC 15408-1:2022 Information security, cybersecurity and privacy protection — Evaluation criteria for IT security — Part 1: Introduction and general model.
- [35] ISO/IEC 18045:2008 Information technology — Security techniques — Methodology for IT security evaluation.
- [36] DoD Enterprise DevSecOps, https://dodcio.defense.gov/Portals/0 /Documents/Library/DoD%20 Enterprise%20DevSecOps%20 Fundamentals%20v2.5.pdf.

- [37] C. Easttom, Computer Security Fundamentals, 4th Edition, Pearson IT Certification, 2019.
- [38] Y. Diogenes and E. Ozkaya, Cybersecurity — Attack and Defense Strategies, Second Edition, Packt Publishing, 2019.
- [39] C. Chio and D. Freeman, Machine Learning and Security: Protecting Systems with Data and Algorithms, O'Reilly, 2018.